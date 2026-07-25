import { useState, useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { Eye, EyeOff } from "lucide-react";

export default function LoginPage() {
  const { sendOtp, verifyOtp, loginWithGoogle } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [otp, setOtp] = useState("");
  const [step, setStep] = useState<"credentials" | "otp">("credentials");
  const [loading, setLoading] = useState(false);
  const [googleLoading, setGoogleLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [resendCooldown, setResendCooldown] = useState(0);

  // Show toast if Google OAuth redirected back with an error.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const googleError = params.get("google_error");
    if (googleError) {
      toast.error(decodeURIComponent(googleError));
      params.delete("google_error");
      const cleanUrl =
        window.location.pathname + (params.toString() ? "?" + params.toString() : "");
      window.history.replaceState({}, "", cleanUrl);
    }
  }, []);

  const startCooldown = () => {
    setResendCooldown(30);
    const t = setInterval(() => {
      setResendCooldown((v) => {
        if (v <= 1) { clearInterval(t); return 0; }
        return v - 1;
      });
    }, 1000);
  };

  const handleSendOtp = async () => {
    if (!email || !email.includes("@")) {
      toast.error("Please enter a valid email address.");
      return;
    }
    if (!password) {
      toast.error("Please enter your password.");
      return;
    }
    setLoading(true);
    try {
      await sendOtp(email, password);
      setStep("otp");
      startCooldown();
      toast.success("OTP sent! Check your email.");
    } catch (err: any) {
      const msg = (err as Error)?.message || "Failed to send OTP. Please check your credentials.";
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleVerifyOtp = async () => {
    if (otp.length !== 6) {
      toast.error("Please enter the 6-digit OTP.");
      return;
    }
    setLoading(true);
    const ok = await verifyOtp(email, otp);
    setLoading(false);
    if (ok) {
      toast.success("Login successful!");
      navigate("/");
    } else {
      toast.error("Invalid or expired OTP. Please try again.");
    }
  };

  const handleGoogleLogin = () => {
    setGoogleLoading(true);
    loginWithGoogle(); // browser will redirect; loading state gives visual feedback
  };

  return (
    <div className="min-h-screen flex items-center justify-center relative overflow-hidden bg-background">
      {/* Animated background */}
      <style>{`
        @keyframes glow-fade { 0%, 100% { opacity: 0.1; transform: scale(0.8); } 50% { opacity: 0.4; transform: scale(1.1); } }
        @keyframes float-up { 0% { transform: translateY(0); opacity: 0; } 20% { opacity: 1; } 80% { opacity: 1; } 100% { transform: translateY(-80px); opacity: 0; } }
        @keyframes rotate-gentle { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        .glow-box { animation: glow-fade 4s ease-in-out infinite; }
        .float-element { animation: float-up 6s ease-out infinite; }
        .rotate-element { animation: rotate-gentle 30s linear infinite; }
        .google-btn {
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 10px;
          width: 100%;
          padding: 10px 16px;
          border-radius: 8px;
          border: 1.5px solid hsl(var(--border));
          background: hsl(var(--card));
          color: hsl(var(--foreground));
          font-size: 14px;
          font-weight: 500;
          cursor: pointer;
          transition: background 0.15s, box-shadow 0.15s, border-color 0.15s;
          letter-spacing: 0.01em;
        }
        .google-btn:hover:not(:disabled) {
          background: hsl(var(--muted));
          border-color: hsl(var(--primary) / 0.4);
          box-shadow: 0 2px 8px hsl(var(--primary) / 0.08);
        }
        .google-btn:active:not(:disabled) { transform: scale(0.99); }
        .google-btn:disabled { opacity: 0.6; cursor: not-allowed; }
        .divider-line { flex: 1; height: 1px; background: hsl(var(--border)); }
      `}</style>

      <svg
        className="absolute inset-0 w-full h-full"
        viewBox="0 0 1200 800"
        xmlns="http://www.w3.org/2000/svg"
        style={{ pointerEvents: "none", opacity: 0.12 }}
        preserveAspectRatio="xMidYMid slice"
      >
        <rect className="glow-box" x="100" y="150" width="80" height="80" rx="16" fill="none" stroke="#b14f65" strokeWidth="2" style={{ animationDelay: "0s" }} />
        <rect className="glow-box" x="950" y="200" width="100" height="100" rx="20" fill="none" stroke="#a12d52" strokeWidth="2" style={{ animationDelay: "1.5s" }} />
        <rect className="glow-box" x="200" y="600" width="70" height="70" rx="14" fill="none" stroke="#c98c52" strokeWidth="2" style={{ animationDelay: "3s" }} />
        <rect className="glow-box" x="900" y="550" width="90" height="90" rx="18" fill="none" stroke="#b14f65" strokeWidth="2" style={{ animationDelay: "1s" }} />
        <circle className="float-element" cx="300" cy="750" r="4" fill="#a12d52" style={{ animationDelay: "0s" }} />
        <circle className="float-element" cx="600" cy="750" r="3" fill="#b14f65" style={{ animationDelay: "1.5s" }} />
        <circle className="float-element" cx="900" cy="750" r="3.5" fill="#c98c52" style={{ animationDelay: "3s" }} />
        <circle className="float-element" cx="150" cy="750" r="3" fill="#b14f65" style={{ animationDelay: "0.75s" }} />
        <circle className="float-element" cx="1050" cy="750" r="4" fill="#a12d52" style={{ animationDelay: "2.25s" }} />
        <circle className="rotate-element" cx="600" cy="400" r="150" fill="none" stroke="#b14f65" strokeWidth="1" opacity="0.15" />
        <circle className="rotate-element" cx="600" cy="400" r="200" fill="none" stroke="#a12d52" strokeWidth="1" opacity="0.1" style={{ animationDelay: "-15s" }} />
      </svg>

      <div className="relative z-10 w-full max-w-sm mx-4">
        <div className="bg-card rounded-2xl border border-border shadow-2xl p-8">
          {/* Logo */}
          <div className="flex items-center justify-center gap-3 mb-8">
            <img src="/icon.jpg" alt="Design Express" className="w-12 h-12 object-contain rounded-lg" />
            <div>
              <p className="font-semibold text-foreground text-base leading-tight">Design Express</p>
              <p className="text-caption text-xs">Management Portal</p>
            </div>
          </div>

          {step === "credentials" ? (
            <>
              <h1 className="text-xl font-semibold text-foreground mb-1">Welcome back</h1>
              <p className="text-sm text-muted-foreground mb-6">Sign in to your account</p>

              <div className="space-y-4">
                <div>
                  <label className="text-sm font-medium text-foreground">Email Address</label>
                  <Input
                    id="login-email"
                    className="mt-1"
                    type="email"
                    placeholder="you@example.com"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    autoFocus
                  />
                </div>
                <div>
                  <label className="text-sm font-medium text-foreground">Password</label>
                  <div className="relative mt-1">
                    <Input
                      id="login-password"
                      type={showPassword ? "text" : "password"}
                      placeholder="••••••••"
                      value={password}
                      onChange={e => setPassword(e.target.value)}
                      onKeyDown={e => e.key === "Enter" && handleSendOtp()}
                      className="pr-10"
                    />
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                      onClick={() => setShowPassword(v => !v)}
                    >
                      {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                </div>
                <Button id="send-otp-btn" className="w-full" onClick={handleSendOtp} disabled={loading}>
                  {loading ? "Sending OTP..." : "Send OTP"}
                </Button>
              </div>

              <p className="text-center text-sm text-muted-foreground mt-5">Only admins can sign in. Contact your super-admin to create a new admin account.</p>

              {/* Divider */}
              <div className="flex items-center gap-3 my-4">
                <span className="divider-line" />
                <span className="text-xs text-muted-foreground whitespace-nowrap">or continue with</span>
                <span className="divider-line" />
              </div>

              {/* Google Login Button */}
              <button
                id="google-login-btn"
                className="google-btn"
                onClick={handleGoogleLogin}
                disabled={googleLoading}
                type="button"
              >
                {/* Official Google "G" logo */}
                <svg width="18" height="18" viewBox="0 0 18 18" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <path d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.796 2.717v2.258h2.908c1.702-1.567 2.684-3.875 2.684-6.615z" fill="#4285F4" />
                  <path d="M9 18c2.43 0 4.467-.806 5.956-2.184l-2.908-2.258c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332C2.438 15.983 5.482 18 9 18z" fill="#34A853" />
                  <path d="M3.964 10.707c-.18-.54-.282-1.117-.282-1.707s.102-1.167.282-1.707V4.961H.957C.347 6.175 0 7.55 0 9s.348 2.825.957 4.039l3.007-2.332z" fill="#FBBC05" />
                  <path d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0 5.482 0 2.438 2.017.957 4.961L3.964 6.293C4.672 4.166 6.656 3.58 9 3.58z" fill="#EA4335" />
                </svg>
                {googleLoading ? "Redirecting to Google…" : "Continue with Google"}
              </button>
            </>
          ) : (
            <>
              <h1 className="text-xl font-semibold text-foreground mb-1">Enter OTP</h1>
              <p className="text-sm text-muted-foreground mb-6">
                A 6-digit code was sent to{" "}
                <span className="font-medium text-foreground">{email}</span>
              </p>
              <div className="space-y-4">
                <div>
                  <label className="text-sm font-medium text-foreground">One-Time Password</label>
                  <Input
                    id="login-otp"
                    className="mt-1 text-center text-2xl tracking-[0.5em] font-mono"
                    type="text"
                    inputMode="numeric"
                    maxLength={6}
                    placeholder="------"
                    value={otp}
                    onChange={e => setOtp(e.target.value.replace(/\D/g, "").slice(0, 6))}
                    onKeyDown={e => e.key === "Enter" && handleVerifyOtp()}
                    autoFocus
                  />
                </div>
                <Button id="verify-otp-btn" className="w-full" onClick={handleVerifyOtp} disabled={loading}>
                  {loading ? "Verifying..." : "Verify & Login"}
                </Button>
                <div className="flex items-center justify-between">
                  <button
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                    onClick={() => { setStep("credentials"); setOtp(""); }}
                  >
                    ← Back
                  </button>
                  <button
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    disabled={resendCooldown > 0 || loading}
                    onClick={handleSendOtp}
                  >
                    {resendCooldown > 0 ? `Resend in ${resendCooldown}s` : "Resend OTP"}
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
        <p className="text-center text-caption mt-4 text-xs">Design Express · Authorised Access Only</p>
      </div>
    </div>
  );
}
