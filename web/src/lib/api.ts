import axios from "axios";

// Access token lives in memory only — it is never written to localStorage.
// This means the token is lost on page refresh, which is intentional.
// Session is restored on mount via the httpOnly refresh token cookie.
let _token = "";

export const getToken = () => _token;
export const setToken = (t: string) => { _token = t; };
export const clearToken = () => { _token = ""; };

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? "/api/v1",
  withCredentials: true, // send the httpOnly refresh_token cookie on every request
});

// Attach access token to every request
api.interceptors.request.use((config) => {
  if (_token) config.headers.Authorization = `Bearer ${_token}`;
  return config;
});

// On 401: try to get a new access token via the refresh cookie, then retry.
// If the refresh also fails, clear state and redirect to /login.
let _refreshing = false;
let _waitQueue: Array<{ resolve: (t: string) => void; reject: (e: unknown) => void }> = [];

const drainQueue = (err: unknown, token: string | null) => {
  _waitQueue.forEach(({ resolve, reject }) => (err ? reject(err) : resolve(token!)));
  _waitQueue = [];
};

api.interceptors.response.use(
  (res) => res,
  async (err) => {
    const original = err.config;

    // Only attempt refresh on 401, and never retry the refresh call itself.
    if (err.response?.status !== 401 || original._retry || original.url === '/auth/refresh') {
      return Promise.reject(err);
    }

    if (_refreshing) {
      // Another request is already refreshing — queue this one.
      return new Promise((resolve, reject) => {
        _waitQueue.push({ resolve, reject });
      }).then((token) => {
        original._retry = true;
        original.headers.Authorization = `Bearer ${token}`;
        return api(original);
      });
    }

    original._retry = true;
    _refreshing = true;

    try {
      const res = await api.post("/auth/refresh");
      const newToken: string = res.data.data.token;
      setToken(newToken);
      drainQueue(null, newToken);
      original.headers.Authorization = `Bearer ${newToken}`;
      return api(original);
    } catch (refreshErr) {
      drainQueue(refreshErr, null);
      clearToken();
      window.location.href = "/login";
      return Promise.reject(refreshErr);
    } finally {
      _refreshing = false;
    }
  }
);
