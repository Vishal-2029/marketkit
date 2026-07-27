package email

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/marketkit/api/internal/config"
	"gopkg.in/gomail.v2"
)

// stripCRLF removes header-injection characters from a value before it's
// used as an email header (e.g. Subject built from a user-supplied name or
// community post title). gomail writes header values verbatim, so an
// unstripped "\r\nBcc: attacker@evil.com" would let the sender smuggle extra
// headers into the message.
func stripCRLF(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

var (
	dialer     *gomail.Dialer
	dialerOnce sync.Once
)

func getDialer() *gomail.Dialer {
	dialerOnce.Do(func() {
		cfg := config.App
		d := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)
		d.SSL = cfg.SMTPSecure
		dialer = d
	})
	return dialer
}

// sendTemplate loads a template file, executes it with data, and sends the email.
func sendTemplate(to, subject, tmplFile string, data interface{}) error {
	return sendTemplateWithAttachments(to, subject, tmplFile, data)
}

// EmailAttachment is an in-memory file attached via gomail.SetCopyFunc.
type EmailAttachment struct {
	Filename string
	Data     []byte
}

// sendTemplateWithAttachments is like sendTemplate but attaches zero or more
// in-memory files. Existing callers of sendTemplate are unaffected.
func sendTemplateWithAttachments(to, subject, tmplFile string, data interface{}, attachments ...EmailAttachment) error {
	_, filename, _, _ := runtime.Caller(0)
	tmplPath := filepath.Join(filepath.Dir(filename), "templates", tmplFile)

	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", config.App.SMTPFrom)
	m.SetHeader("To", stripCRLF(to))
	m.SetHeader("Subject", stripCRLF(subject))
	m.SetBody("text/html", buf.String())
	for _, a := range attachments {
		if len(a.Data) == 0 || a.Filename == "" {
			continue
		}
		data := a.Data
		name := a.Filename
		m.Attach(name, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		}))
	}

	return getDialer().DialAndSend(m)
}

// FormatAmount converts paise to a formatted rupee string e.g. "₹4,999".
func FormatAmount(paise int64) string {
	rupees := paise / 100
	paise2 := paise % 100
	if paise2 == 0 {
		return fmt.Sprintf("₹%d", rupees)
	}
	return fmt.Sprintf("₹%d.%02d", rupees, paise2)
}

// FormatDate formats a time.Time as "02 Jan 2006".
func FormatDate(t time.Time) string {
	return t.Format("02 Jan 2006")
}

// ─── Data structs ─────────────────────────────────────────────────────────────

type otpTemplateData struct {
	Name string
	OTP  string
	Year int
}

type welcomeTemplateData struct {
	Name    string
	IsAdmin bool
	Year    int
}

// welcomeModeTemplateData is shared by the mode-specific welcome emails sent
// on first-ever login, once the account's chosen app mode is known.
type welcomeModeTemplateData struct {
	Name string
	Year int
}

// PaymentReceiptData holds data for the payment receipt email sent to users.
type PaymentReceiptData struct {
	Name          string
	PlanName      string
	Amount        string
	TransactionID string
	Provider      string
	PaidAt        string
	ExpiresAt     string
	Year          int
}

// SubscriptionEmailData is used for both new-subscription and plan-upgrade emails.
type SubscriptionEmailData struct {
	Name      string
	PlanName  string
	ExpiresAt string
	Features  []string
	IsUpgrade bool // true → "Plan Upgraded", false → "Subscription Activated"
	Year      int
}

// ExpiryEmailData is used for both the 7-day warning and the expired email.
type ExpiryEmailData struct {
	Name      string
	PlanName  string
	ExpiresAt string
	DaysLeft  int // > 0 for warning, 0 for expired
	Year      int
}

// AdminSubAlertData is the payload for admin new-subscription/upgrade notifications.
type AdminSubAlertData struct {
	UserName  string
	UserEmail string
	PlanName  string
	Amount    string
	Provider  string
	PaidAt    string
	IsUpgrade bool
	Year      int
}

// RefundEmailData holds data for the refund confirmation email sent to users.
type RefundEmailData struct {
	Name          string
	PlanName      string
	Amount        string
	TransactionID string
	RefundID      string
	Reason        string
	RefundedAt    string
	Year          int
}

// ─── Send functions ───────────────────────────────────────────────────────────

// SendOTPEmail sends a one-time password to the given address.
func SendOTPEmail(to, name, otp string) error {
	if name == "" {
		name = "there"
	}
	return sendTemplate(to, "Your OTP Code — MarketKit", "otp.html",
		otpTemplateData{Name: name, OTP: otp, Year: time.Now().Year()})
}

// SendWelcomeEmail fires on first-ever login for both admins (isAdmin=true) and
// users whose account predates app-mode selection (CurrentAppMode == "").
// Mode-aware accounts get SendWelcomeLearningEmail / SendWelcomeMarketEmail
// instead — see completeLogin in user_auth/service.go.
func SendWelcomeEmail(to, name string, isAdmin bool) error {
	if name == "" {
		name = "there"
	}
	subject := "Welcome to MarketKit"
	if isAdmin {
		subject = "Welcome to MarketKit Admin Panel"
	}
	return sendTemplate(to, subject, "welcome.html",
		welcomeTemplateData{Name: name, IsAdmin: isAdmin, Year: time.Now().Year()})
}

// SendWelcomeLearningEmail fires on first-ever login for users who chose the
// Learning mode (at registration, or by switching to it later).
func SendWelcomeLearningEmail(to, name string) error {
	if name == "" {
		name = "there"
	}
	return sendTemplate(to, "Welcome to Learning — MarketKit", "welcome_learning.html",
		welcomeModeTemplateData{Name: name, Year: time.Now().Year()})
}

// SendWelcomeMarketEmail fires on first-ever login for users who chose the
// Product Market mode (at registration, or by switching to it later).
func SendWelcomeMarketEmail(to, name string) error {
	if name == "" {
		name = "there"
	}
	return sendTemplate(to, "Welcome to the Product Market — MarketKit", "welcome_market.html",
		welcomeModeTemplateData{Name: name, Year: time.Now().Year()})
}

// SendPaymentReceiptEmail sends a formal payment receipt to the user.
func SendPaymentReceiptEmail(to string, data PaymentReceiptData) error {
	data.Year = time.Now().Year()
	return sendTemplate(to, "Payment Receipt — MarketKit", "payment_receipt.html", data)
}

// MarketPurchaseEmailData is the payload for Product Market purchase thank-you emails.
type MarketPurchaseEmailData struct {
	Name         string
	ProductTitle string
	SellerName   string
	Amount       string
	PaidVia      string
	PaidAt       string
	PurchaseID   string
	PreviewURL   string
	DownloadLink string // set when the product file is too large to attach
	Year         int
}

// SendMarketPurchaseEmail emails the buyer a thank-you with the invoice PDF
// and (when small enough) the product file attached. If productFile is empty
// and DownloadLink is set on data, the template shows a secure download link.
func SendMarketPurchaseEmail(to string, data MarketPurchaseEmailData, invoicePDF []byte, productFile []byte, productFileName string) error {
	data.Year = time.Now().Year()
	if data.Name == "" {
		data.Name = "there"
	}
	atts := []EmailAttachment{
		{Filename: fmt.Sprintf("invoice-%s.pdf", data.PurchaseID), Data: invoicePDF},
	}
	if len(productFile) > 0 && productFileName != "" {
		atts = append(atts, EmailAttachment{Filename: productFileName, Data: productFile})
	}
	return sendTemplateWithAttachments(to, "Thank you for your purchase — MarketKit",
		"market_purchase_receipt.html", data, atts...)
}

// SendNewSubscriptionEmail sends a subscription-activated confirmation to the user.
func SendNewSubscriptionEmail(to string, data SubscriptionEmailData) error {
	data.IsUpgrade = false
	data.Year = time.Now().Year()
	return sendTemplate(to, "Your Subscription is Active — MarketKit", "subscription_confirm.html", data)
}

// SendPlanUpgradeEmail sends a plan-upgraded confirmation to the user.
func SendPlanUpgradeEmail(to string, data SubscriptionEmailData) error {
	data.IsUpgrade = true
	data.Year = time.Now().Year()
	return sendTemplate(to, "Your Plan Has Been Upgraded — MarketKit", "subscription_confirm.html", data)
}

// SendExpiryWarningEmail notifies the user that their plan expires in a few days.
func SendExpiryWarningEmail(to string, data ExpiryEmailData) error {
	data.Year = time.Now().Year()
	subject := fmt.Sprintf("Your Plan Expires in %d Days — MarketKit", data.DaysLeft)
	return sendTemplate(to, subject, "expiry_warning.html", data)
}

// SendPlanExpiredEmail notifies the user that their plan has expired.
func SendPlanExpiredEmail(to string, data ExpiryEmailData) error {
	data.Year = time.Now().Year()
	return sendTemplate(to, "Your Plan Has Expired — MarketKit", "subscription_expired.html", data)
}

// SendAccountSuspendedEmail notifies a user that their account has been suspended.
func SendAccountSuspendedEmail(to, name string) error {
	if name == "" {
		name = "there"
	}
	return sendTemplate(to, "Your Account Has Been Suspended — MarketKit", "account_suspended.html",
		struct {
			Name string
			Year int
		}{Name: name, Year: time.Now().Year()})
}

// SendRefundEmail notifies a user that their payment has been refunded.
func SendRefundEmail(to string, data RefundEmailData) error {
	data.Year = time.Now().Year()
	return sendTemplate(to, "Your Refund Has Been Processed — MarketKit", "refund.html", data)
}

// RefundRejectionData holds data for the refund-rejection email sent to users.
type RefundRejectionData struct {
	Name     string
	PlanName string
	Reason   string
	Note     string
	Year     int
}

// SendRefundRejectionEmail notifies a user that their refund request was declined.
func SendRefundRejectionEmail(to string, data RefundRejectionData) error {
	data.Year = time.Now().Year()
	return sendTemplate(to, "Update on Your Refund Request — MarketKit", "refund_rejected.html", data)
}

// SendAdminSubAlert sends a new-subscription or plan-upgrade alert to the admin.
func SendAdminSubAlert(to string, data AdminSubAlertData) error {
	data.Year = time.Now().Year()
	subject := "New Subscription — " + data.UserName
	if data.IsUpgrade {
		subject = "Plan Upgraded — " + data.UserName
	}
	return sendTemplate(to, subject, "admin_sub_alert.html", data)
}

// AdminPostAlertData is the payload for the admin new-community-post notification.
type AdminPostAlertData struct {
	Author   string
	Category string
	Title    string
	Content  string
	PostedAt string
	Year     int
}

// SendAdminCommunityPostAlert notifies the admin that a user published a community post.
func SendAdminCommunityPostAlert(to string, data AdminPostAlertData) error {
	data.Year = time.Now().Year()
	return sendTemplate(to, "New Community Post — "+data.Title, "admin_post_alert.html", data)
}
