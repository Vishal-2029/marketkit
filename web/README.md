# Studio Manager — Admin Panel

React admin panel for the Design Express embroidery learning platform. Admins manage videos, users, subscription plans, payments, and monitor platform analytics.

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Framework | React 18 |
| Language | TypeScript 5 |
| Build tool | Vite 5 |
| Styling | Tailwind CSS 3 |
| Components | shadcn/ui (Radix UI) |
| Data fetching | TanStack Query v5 |
| HTTP | Axios |
| Forms | React Hook Form + Zod |
| Charts | Recharts |
| Icons | Lucide React |
| Toasts | Sonner |
| Routing | React Router v6 |

---

## Setup

### Prerequisites

- Node.js 18+

### Run dev server

```bash
cd "web "
npm install
npm run dev
```

Admin panel opens at `http://localhost:5173`.

### Build for production

```bash
npm run build         # outputs to dist/
npm run preview       # preview production build locally
```

### Run tests

```bash
npm run test          # Vitest
```

---

## Environment Variables

Create `web /.env.local`:

```env
VITE_API_URL=http://localhost:3000/api/v1
```

---

## Project Structure

```
web /src/
├── components/
│   ├── ui/             # shadcn/ui base components
│   ├── AdminLayout.tsx # Sidebar + top nav wrapper
│   ├── NavLink.tsx
│   ├── PageHeader.tsx
│   ├── StatCard.tsx    # KPI metric card
│   └── StatusBadge.tsx
├── contexts/           # React contexts (auth)
├── hooks/              # Custom React Query hooks per resource
├── lib/
│   └── api.ts          # Axios instance with interceptors
├── pages/
│   ├── DashboardPage.tsx
│   ├── UsersPage.tsx
│   ├── VideosPage.tsx
│   ├── PhotosPage.tsx
│   ├── PlansPage.tsx
│   ├── PaymentsPage.tsx
│   ├── SessionsPage.tsx
│   ├── AuditLogsPage.tsx
│   ├── PlaybackPage.tsx
│   ├── RevenuePage.tsx
│   ├── LoginPage.tsx
│   ├── RegisterPage.tsx
│   └── NotFound.tsx
├── services/           # API call functions per domain
├── test/               # Vitest test files
└── App.tsx             # Router + layout wiring
```

---

## Pages

| Route | Page | Description |
|-------|------|-------------|
| `/` | Dashboard | KPI cards, revenue chart, recent activity |
| `/users` | Users | User list, search, status filter, plan assignment |
| `/videos` | Videos | Video upload, status (DRAFT→PROCESSING→PUBLISHED), metadata |
| `/photos` | Photos | Photo upload and management |
| `/plans` | Plans | Create and configure subscription plans |
| `/payments` | Payments | Razorpay + manual payment records, activate pending |
| `/sessions` | Sessions | Admin session list |
| `/audit-logs` | Audit Logs | Append-only log of admin actions |
| `/playback` | Playback | Video play statistics |
| `/revenue` | Revenue | Revenue analytics and charts |
| `/login` | Login | OTP-based admin login |
| `/register` | Register | Admin account creation |

---

## Authentication

The admin panel uses OTP-based login:

1. Enter email on `/login` → backend sends OTP email
2. Enter OTP → receive short-lived access token + httpOnly refresh cookie
3. Access token stored **in memory only** (lost on page refresh by design)
4. On page reload: Axios instance automatically calls `POST /auth/refresh` using the httpOnly cookie to restore session without requiring re-login

The Axios interceptor handles 401 errors automatically:
- Queues all concurrent failed requests
- Calls `/auth/refresh` once
- Retries all queued requests with the new token

---

## Data Fetching

All server state is managed by TanStack Query:

- `staleTime: 30_000` (30 seconds)
- `retry: 1`
- `refetchOnWindowFocus: false`

Custom hooks in `src/hooks/` wrap `useQuery` and `useMutation` calls for each resource, keeping page components clean.

---

## Available Scripts

| Script | Description |
|--------|-------------|
| `npm run dev` | Start Vite dev server on port 5173 |
| `npm run build` | TypeScript compile + Vite production build |
| `npm run preview` | Serve production build locally |
| `npm run lint` | ESLint |
| `npm run test` | Vitest |
| `npm run type-check` | TypeScript type check without emitting |

Or from the repo root:

```bash
make web-dev     # same as npm run dev
make web-build   # same as npm run build
```
