import axios from 'axios';

const client = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,
});

let csrfToken = '';

client.interceptors.request.use((config) => {
  if (csrfToken && config.method !== 'get') {
    config.headers['X-CSRF-Token'] = csrfToken;
  }
  return config;
});

client.interceptors.response.use(
  (response) => {
    const token = response.headers['x-csrf-token'];
    if (token) {
      csrfToken = token;
    }
    return response;
  },
  (error) => {
    if (error.response?.status === 401 && !window.location.pathname.startsWith('/login')) {
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export async function fetchCSRFToken() {
  const res = await client.get('/auth/csrf');
  csrfToken = res.data.csrf_token;
}

export { client, csrfToken };
