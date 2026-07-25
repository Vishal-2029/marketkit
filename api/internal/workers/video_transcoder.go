package workers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
)

// VideoTranscoder polls for PROCESSING videos and converts them to multi-quality HLS.
// It runs one transcode job at a time to avoid overloading the server.
type VideoTranscoder struct {
	cfg *config.Config
}

func NewVideoTranscoder(cfg *config.Config) *VideoTranscoder {
	return &VideoTranscoder{cfg: cfg}
}

func (t *VideoTranscoder) Start() {
	// Process any queued videos immediately on startup, then on every tick.
	t.processNext()
	ticker := time.NewTicker(30 * time.Second)
	slog.Info("[transcoder] started, polling every 30s")
	for range ticker.C {
		t.processNext()
	}
}

func (t *VideoTranscoder) processNext() {
	var v models.Video
	if err := database.DB.
		Where("status = ?", models.VideoStatusProcessing).
		Order("uploaded_at ASC").
		First(&v).Error; err != nil {
		return // nothing queued
	}

	slog.Info("[transcoder] starting", "id", v.ID, "title", v.Title)
	// Record when work began so the admin panel can show per-video transcode
	// time; cleared finish time also marks "in progress" on a retry.
	database.DB.Model(&v).Updates(map[string]interface{}{
		"transcode_started_at":  time.Now(),
		"transcode_finished_at": nil,
	})
	if err := t.transcode(v); err != nil {
		slog.Error("[transcoder] failed", "id", v.ID, "error", err)
		errMsg := err.Error()
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
		database.DB.Model(&v).Updates(map[string]interface{}{
			"status":          models.VideoStatusError,
			"transcode_error": errMsg,
		})
	}
}

func (t *VideoTranscoder) transcode(v models.Video) error {
	tempDir := filepath.Join(os.TempDir(), "transcode_"+v.ID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	ext := filepath.Ext(v.FileKey)
	if ext == "" {
		ext = ".mp4"
	}
	inputPath := filepath.Join(tempDir, "input"+ext)

	if err := t.downloadInput(v, inputPath); err != nil {
		return fmt.Errorf("download input: %w", err)
	}

	durationSec := probeDuration(inputPath)
	hasAudio := probeHasAudio(inputPath)

	hlsPublicBase := t.computeHLSBase(v.ID)
	if hlsPublicBase == "" {
		return fmt.Errorf("public URL unavailable — set R2_PUBLIC_BASE or SERVER_BASE_URL to enable HLS")
	}

	if err := t.runFFmpeg(inputPath, tempDir, hasAudio); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	if err := t.uploadHLS(tempDir, v.ID, hlsPublicBase); err != nil {
		return fmt.Errorf("upload hls: %w", err)
	}

	mp4Keys, mp4Sizes, err := t.buildAndUploadMP4s(tempDir, v.ID)
	if err != nil {
		// Non-fatal: HLS streaming still works; log and continue.
		slog.Warn("[transcoder] mp4 build failed, download quality unavailable", "id", v.ID, "error", err)
	}

	hlsKey := fmt.Sprintf("videos/hls/%s/index.m3u8", v.ID)
	// Column names must match the schema GORM auto-migrated from the model:
	// MP4Key1080p → mp4_key1080p (no underscore before the tier). With the
	// underscored spelling Postgres rejects the whole UPDATE, the video never
	// publishes, and the original upload below gets deleted anyway — every
	// upload ends up ERROR with its source gone.
	updates := map[string]interface{}{
		"status":                models.VideoStatusPublished,
		"hls_key":               hlsKey,
		"duration_seconds":      durationSec,
		"transcode_error":       "",
		"transcode_finished_at": time.Now(),
		"mp4_key1080p":          mp4Keys["1080p"],
		"mp4_key720p":           mp4Keys["720p"],
		"mp4_key480p":           mp4Keys["480p"],
		"mp4_key360p":           mp4Keys["360p"],
		"mp4_key240p":           mp4Keys["240p"],
		"mp4_size1080p":         mp4Sizes["1080p"],
		"mp4_size720p":          mp4Sizes["720p"],
		"mp4_size480p":          mp4Sizes["480p"],
		"mp4_size360p":          mp4Sizes["360p"],
		"mp4_size240p":          mp4Sizes["240p"],
	}
	// The original upload is deleted right after this, so a failed publish
	// write must abort the job — otherwise the video is unrecoverable.
	if err := database.DB.Model(&v).Updates(updates).Error; err != nil {
		return fmt.Errorf("publish update: %w", err)
	}
	v.Status = models.VideoStatusPublished
	database.AssignVideoToDefaultPlaylist(&v)

	// Delete the original raw upload — HLS variants are now the source of truth.
	// This avoids storing both the uncompressed original and all transcoded copies.
	if v.FileKey != "" {
		if err := storage.Store.Delete(context.Background(), v.FileKey); err != nil {
			slog.Warn("[transcoder] failed to delete original file", "key", v.FileKey, "error", err)
		} else {
			database.DB.Model(&v).Update("file_key", "")
		}
	}

	slog.Info("[transcoder] done", "id", v.ID, "duration_s", durationSec)
	return nil
}

// hlsVariants defines the six HLS quality tiers produced during transcoding.
// v0=1080p … v5=144p. min(Xp, ih) prevents upscaling low-resolution sources.
var hlsVariants = []struct {
	dir     string
	height  string
	crf     string
	maxrate string
	bufsize string
}{
	{"v0", "1080", "21", "4000k", "8000k"},
	{"v1", "720", "22", "2800k", "5600k"},
	{"v2", "480", "23", "1400k", "2800k"},
	{"v3", "360", "24", "700k", "1400k"},
	{"v4", "240", "25", "400k", "800k"},
	{"v5", "144", "28", "200k", "400k"},
}

func (t *VideoTranscoder) runFFmpeg(inputPath, tempDir string, hasAudio bool) error {
	for _, v := range hlsVariants {
		os.MkdirAll(filepath.Join(tempDir, v.dir), 0755)
	}

	n := len(hlsVariants)
	// Build split filter: [0:v]split=6[va][vb]…
	splitLabels := make([]string, n)
	for i := range hlsVariants {
		splitLabels[i] = fmt.Sprintf("[v%c]", rune('a'+i))
	}
	filterComplex := fmt.Sprintf("[0:v]split=%d%s", n, strings.Join(splitLabels, ""))
	for i, hv := range hlsVariants {
		filterComplex += fmt.Sprintf(";[v%c]scale=w=-2:h='min(%s,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2[v%cout]",
			rune('a'+i), hv.height, rune('a'+i))
	}

	args := []string{"-y", "-i", inputPath, "-filter_complex", filterComplex}

	// Thread budget per rendition. The whole ffmpeg pipeline runs at the speed
	// of its slowest encoder, and the 1080p rendition costs roughly as much as
	// all the others combined — pinning every encoder to 1 thread leaves the
	// job bottlenecked on one core no matter how many are free. Give the two
	// heavy renditions a share of the available cores and keep the small ones
	// at 1. GOMAXPROCS is cgroup-aware (Go 1.25+), so a docker `cpus:` cap on
	// the api service shrinks the budget instead of oversubscribing the host.
	budget := runtime.GOMAXPROCS(0)
	threadsFor := func(i int) string {
		switch i {
		case 0: // 1080p
			return strconv.Itoa(max(1, budget/2))
		case 1: // 720p
			return strconv.Itoa(max(1, budget/4))
		default:
			return "1"
		}
	}

	for i, hv := range hlsVariants {
		label := fmt.Sprintf("[v%cout]", rune('a'+i))
		si := fmt.Sprintf("%d", i)
		args = append(args,
			"-map", label,
			"-c:v:"+si, "libx264",
			"-crf:v:"+si, hv.crf,
			"-preset:v:"+si, "veryfast",
			"-threads:v:"+si, threadsFor(i),
			"-maxrate:v:"+si, hv.maxrate,
			"-bufsize:v:"+si, hv.bufsize,
		)
	}

	if hasAudio {
		for range hlsVariants {
			args = append(args, "-map", "0:a:0")
		}
		args = append(args, "-c:a", "aac", "-b:a", "128k")
		var streamMap string
		for i := range hlsVariants {
			if i > 0 {
				streamMap += " "
			}
			streamMap += fmt.Sprintf("v:%d,a:%d", i, i)
		}
		args = append(args, "-var_stream_map", streamMap)
	} else {
		var streamMap string
		for i := range hlsVariants {
			if i > 0 {
				streamMap += " "
			}
			streamMap += fmt.Sprintf("v:%d", i)
		}
		args = append(args, "-var_stream_map", streamMap)
	}

	args = append(args,
		// Keep source timestamps instead of synthesizing a constant frame
		// rate. Browser screen recordings (webm) carry no fps metadata, so
		// without this ffmpeg falls back to the container's 1000Hz timebase
		// and pads the output to ~1000fps with duplicate frames — >99% of
		// encoded frames were dups, making transcodes ~16x slower and fatter.
		"-fps_mode", "vfr",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-master_pl_name", "index.m3u8",
		"-hls_segment_filename", filepath.Join(tempDir, "v%v", "seg%04d.ts"),
		filepath.Join(tempDir, "v%v", "index.m3u8"),
	)

	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if len(msg) > 800 {
			msg = "..." + msg[len(msg)-800:]
		}
		return fmt.Errorf("exit %v: %s", err, msg)
	}
	return nil
}

// mp4VariantMap maps HLS variant directory to the download quality label.
// v5 (144p) is streaming-only and excluded from downloads.
var mp4VariantMap = []struct {
	dir     string
	quality string
}{
	{"v0", "1080p"},
	{"v1", "720p"},
	{"v2", "480p"},
	{"v3", "360p"},
	{"v4", "240p"},
}

// buildAndUploadMP4s remuxes each HLS variant's .ts segments into a single MP4
// (copy-only, no re-encode) and uploads them. Returns map[quality]storageKey.
func (t *VideoTranscoder) buildAndUploadMP4s(tempDir, videoID string) (map[string]string, map[string]int64, error) {
	ctx := context.Background()
	keys := make(map[string]string)
	sizes := make(map[string]int64)

	for _, vm := range mp4VariantMap {
		variantDir := filepath.Join(tempDir, vm.dir)
		entries, err := os.ReadDir(variantDir)
		if err != nil {
			return keys, sizes, fmt.Errorf("read variant dir %s: %w", vm.dir, err)
		}

		var segments []string
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".ts") {
				segments = append(segments, filepath.Join(variantDir, e.Name()))
			}
		}
		if len(segments) == 0 {
			continue
		}

		outPath := filepath.Join(tempDir, vm.quality+".mp4")
		concatStr := "concat:" + strings.Join(segments, "|")
		cmd := exec.Command("ffmpeg", "-y", "-i", concatStr, "-c", "copy", outPath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			slog.Warn("[transcoder] mp4 remux failed", "quality", vm.quality, "error", stderr.String())
			continue
		}

		storageKey := fmt.Sprintf("videos/mp4/%s/%s.mp4", videoID, vm.quality)
		size, err := uploadFile(ctx, outPath, storageKey, "video/mp4")
		if err != nil {
			slog.Warn("[transcoder] mp4 upload failed", "quality", vm.quality, "error", err)
			continue
		}
		keys[vm.quality] = storageKey
		sizes[vm.quality] = size
		os.Remove(outPath)
	}

	return keys, sizes, nil
}

func (t *VideoTranscoder) uploadHLS(tempDir, videoID, hlsPublicBase string) error {
	ctx := context.Background()

	allVariants := make([]string, len(hlsVariants))
	for i, hv := range hlsVariants {
		allVariants[i] = hv.dir
	}

	for _, variant := range allVariants {
		variantDir := filepath.Join(tempDir, variant)
		variantBase := hlsPublicBase + "/" + variant

		// Upload .ts segments
		entries, err := os.ReadDir(variantDir)
		if err != nil {
			return fmt.Errorf("read variant dir %s: %w", variant, err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".ts") {
				continue
			}
			key := fmt.Sprintf("videos/hls/%s/%s/%s", videoID, variant, e.Name())
			if _, err := uploadFile(ctx, filepath.Join(variantDir, e.Name()), key, "video/mp2t"); err != nil {
				return fmt.Errorf("upload segment %s/%s: %w", variant, e.Name(), err)
			}
		}

		// Rewrite and upload variant playlist
		varManifest, err := os.ReadFile(filepath.Join(variantDir, "index.m3u8"))
		if err != nil {
			return fmt.Errorf("read variant manifest %s: %w", variant, err)
		}
		rewritten := rewriteM3U8(string(varManifest), variantBase)
		key := fmt.Sprintf("videos/hls/%s/%s/index.m3u8", videoID, variant)
		if err := uploadBytes(ctx, []byte(rewritten), key, "application/vnd.apple.mpegurl"); err != nil {
			return fmt.Errorf("upload variant manifest %s: %w", variant, err)
		}
	}

	// Rewrite and upload master playlist
	masterContent, err := os.ReadFile(filepath.Join(tempDir, "index.m3u8"))
	if err != nil {
		return fmt.Errorf("read master manifest: %w", err)
	}
	rewritten := rewriteM3U8(string(masterContent), hlsPublicBase)
	masterKey := fmt.Sprintf("videos/hls/%s/index.m3u8", videoID)
	if err := uploadBytes(ctx, []byte(rewritten), masterKey, "application/vnd.apple.mpegurl"); err != nil {
		return fmt.Errorf("upload master manifest: %w", err)
	}

	return nil
}

// rewriteM3U8 turns relative URI lines into absolute URLs by prepending baseURL.
func rewriteM3U8(content, baseURL string) string {
	sc := bufio.NewScanner(strings.NewReader(content))
	var out strings.Builder
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			out.WriteString(line + "\n")
			continue
		}
		out.WriteString(baseURL + "/" + trimmed + "\n")
	}
	return out.String()
}

// computeHLSBase derives the public base URL for a video's HLS directory.
func (t *VideoTranscoder) computeHLSBase(videoID string) string {
	placeholder := fmt.Sprintf("videos/hls/%s/placeholder.ts", videoID)
	url := storage.Store.PublicURL(placeholder)
	if url == "" {
		return ""
	}
	return strings.TrimSuffix(url, "/placeholder.ts")
}

func (t *VideoTranscoder) downloadInput(v models.Video, destPath string) error {
	// Prefer direct disk read for local storage (avoids HTTP overhead)
	localPath := filepath.Join(t.cfg.UploadDir, v.FileKey)
	if _, err := os.Stat(localPath); err == nil {
		return copyFile(localPath, destPath)
	}
	// Download from cloud storage via signed URL
	signedURL, err := storage.Store.SignedURL(context.Background(), v.FileKey, 2*time.Hour)
	if err != nil {
		return fmt.Errorf("get signed url: %w", err)
	}
	return downloadHTTP(signedURL, destPath)
}

func probeDuration(inputPath string) int {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath).Output()
	if err != nil {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return int(f)
}

func probeHasAudio(inputPath string) bool {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
		inputPath).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func uploadFile(ctx context.Context, localPath, key, contentType string) (int64, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if err := storage.Store.Upload(ctx, key, contentType, f, info.Size()); err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func uploadBytes(ctx context.Context, data []byte, key, contentType string) error {
	return storage.Store.Upload(ctx, key, contentType, bytes.NewReader(data), int64(len(data)))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func downloadHTTP(url, destPath string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d downloading video", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}
