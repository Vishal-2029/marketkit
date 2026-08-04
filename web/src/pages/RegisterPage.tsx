import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { Eye, EyeOff } from "lucide-react";

export default function RegisterPage() {
  const { register } = useAuth();
  const navigate = useNavigate();

  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleRegister = async () => {
    if (!firstName.trim()) { toast.error("First name is required."); return; }
    if (!email || !email.includes("@")) { toast.error("Please enter a valid email address."); return; }
    if (password.length < 6) { toast.error("Password must be at least 6 characters."); return; }

    setLoading(true);
    try {
      await register(firstName, lastName, phone, email, password);
      toast.success("Account created! Check your email for the OTP, then log in.");
      navigate("/login");
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg ?? "Registration failed. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center relative overflow-hidden bg-background">
      {/* Simple logo-inspired animated background */}
      <style>{`
        @keyframes glow-fade { 0%, 100% { opacity: 0.1; transform: scale(0.8); } 50% { opacity: 0.4; transform: scale(1.1); } }
        @keyframes float-up { 0% { transform: translateY(0); opacity: 0; } 20% { opacity: 1; } 80% { opacity: 1; } 100% { transform: translateY(-80px); opacity: 0; } }
        @keyframes rotate-gentle { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        .glow-box { animation: glow-fade 4s ease-in-out infinite; }
        .float-element { animation: float-up 6s ease-out infinite; }
        .rotate-element { animation: rotate-gentle 30s linear infinite; }
      `}</style>
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox="0 0 1200 800"
        xmlns="http://www.w3.org/2000/svg"
        style={{ pointerEvents: "none", opacity: 0.12 }}
        preserveAspectRatio="xMidYMid slice"
      >
        {/* Glowing rounded squares - inspired by logo */}
        <rect className="glow-box" x="100" y="150" width="80" height="80" rx="16" fill="none" stroke="#818CF8" strokeWidth="2" style={{ animationDelay: '0s' }} />
        <rect className="glow-box" x="950" y="200" width="100" height="100" rx="20" fill="none" stroke="#4F46E5" strokeWidth="2" style={{ animationDelay: '1.5s' }} />
        <rect className="glow-box" x="200" y="600" width="70" height="70" rx="14" fill="none" stroke="#A5B4FC" strokeWidth="2" style={{ animationDelay: '3s' }} />
        <rect className="glow-box" x="900" y="550" width="90" height="90" rx="18" fill="none" stroke="#818CF8" strokeWidth="2" style={{ animationDelay: '1s' }} />
        
        {/* Floating particles with staggered timing */}
        <circle className="float-element" cx="300" cy="750" r="4" fill="#4F46E5" style={{ animationDelay: '0s' }} />
        <circle className="float-element" cx="600" cy="750" r="3" fill="#818CF8" style={{ animationDelay: '1.5s' }} />
        <circle className="float-element" cx="900" cy="750" r="3.5" fill="#A5B4FC" style={{ animationDelay: '3s' }} />
        <circle className="float-element" cx="150" cy="750" r="3" fill="#818CF8" style={{ animationDelay: '0.75s' }} />
        <circle className="float-element" cx="1050" cy="750" r="4" fill="#4F46E5" style={{ animationDelay: '2.25s' }} />
        
        {/* Large rotating decorative circles */}
        <circle className="rotate-element" cx="600" cy="400" r="150" fill="none" stroke="#818CF8" strokeWidth="1" opacity="0.15" />
        <circle className="rotate-element" cx="600" cy="400" r="200" fill="none" stroke="#4F46E5" strokeWidth="1" opacity="0.1" style={{ animationDelay: '-15s' }} />
      </svg>

      <div className="relative z-10 w-full max-w-sm mx-4">
        <div className="bg-card rounded-2xl border border-border shadow-2xl p-8">
          {/* Logo */}
          <div className="flex items-center justify-center gap-3 mb-8">
            <img src="/icon.svg" alt="MarketKit" className="w-12 h-12 object-contain rounded-lg" />
            <div>
              <p className="font-semibold text-foreground text-base leading-tight">MarketKit</p>
              <p className="text-caption text-xs">Management Portal</p>
            </div>
          </div>

          <h1 className="text-xl font-semibold text-foreground mb-1">Create Account</h1>
          <p className="text-sm text-muted-foreground mb-6">Fill in your details to get started</p>

          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-sm font-medium text-foreground">First Name <span className="text-danger">*</span></label>
                <Input
                  className="mt-1"
                  value={firstName}
                  onChange={e => setFirstName(e.target.value)}
                  autoFocus
                />
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Last Name</label>
                <Input
                  className="mt-1"
                  value={lastName}
                  onChange={e => setLastName(e.target.value)}
                />
              </div>
            </div>

            <div>
              <label className="text-sm font-medium text-foreground">Mobile Number</label>
              <Input
                className="mt-1"
                type="tel"
                placeholder="+91"
                value={phone}
                onChange={e => setPhone(e.target.value)}
              />
            </div>

            <div>
              <label className="text-sm font-medium text-foreground">Email Address <span className="text-danger">*</span></label>
              <Input
                className="mt-1"
                type="email"
                placeholder="you@example.com"
                value={email}
                onChange={e => setEmail(e.target.value)}
              />
            </div>

            <div>
              <label className="text-sm font-medium text-foreground">Password <span className="text-danger">*</span></label>
              <div className="relative mt-1">
                <Input
                  type={showPassword ? "text" : "password"}
                  placeholder="Min. 6 characters"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  onKeyDown={e => e.key === "Enter" && handleRegister()}
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

            <Button className="w-full" onClick={handleRegister} disabled={loading}>
              {loading ? "Creating Account..." : "Create Account"}
            </Button>
          </div>

          <p className="text-center text-sm text-muted-foreground mt-5">
            Already have an account?{" "}
            <Link to="/login" className="text-primary font-medium hover:underline">
              Login
            </Link>
          </p>
        </div>
        <p className="text-center text-caption mt-4 text-xs">MarketKit · Authorised Access Only</p>
      </div>
    </div>
  );
}
