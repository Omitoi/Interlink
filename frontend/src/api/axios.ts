// Centralized Axios client with JWT + 401 handling

import axios from 'axios'

// Get the API url from the environmental variable set in .env.local
const baseURL =
  (import.meta.env.VITE_API_BASE_URL as string | undefined)?.replace(/\/+$/, "") ||
  "http://localhost:8080";

const api = axios.create({ baseURL });

// Add the token to every request if it exists
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers = config.headers ?? {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
})

// If backend says 401, clear token and go to login
api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error?.response?.status === 401) {
      localStorage.removeItem("token");
      if (location.pathname !== "/") location.assign("/");
    }
    return Promise.reject(error);
  }
);

export default api;

