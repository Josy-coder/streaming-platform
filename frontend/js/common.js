
const API_URL = 'http://localhost:3000';
const MEDIA_URL = 'http://localhost:8081';
const WS_URL = 'ws://localhost:3000';
function getAuthToken() {
    return localStorage.getItem('token');
}
function getAuthHeaders() {
    const token = getAuthToken();
    return {
        'Authorization': token ? `Bearer ${token}` : '',
        'Content-Type': 'application/json'
    };
}
function isAuthenticated() {
    return !!getAuthToken();
}
function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
}
function formatTimeAgo(dateString) {
    const date = new Date(dateString);
    const now = new Date();
    const seconds = Math.floor((now - date) / 1000);
    if (seconds < 60) {
        return 'just now';
    }
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
        return `${minutes} ${minutes === 1 ? 'minute' : 'minutes'} ago`;
    }
    const hours = Math.floor(minutes / 60);
    if (hours < 24) {
        return `${hours} ${hours === 1 ? 'hour' : 'hours'} ago`;
    }
    const days = Math.floor(hours / 24);
    if (days < 7) {
        return `${days} ${days === 1 ? 'day' : 'days'} ago`;
    }
    const weeks = Math.floor(days / 7);
    if (weeks < 4) {
        return `${weeks} ${weeks === 1 ? 'week' : 'weeks'} ago`;
    }
    const months = Math.floor(days / 30);
    if (months < 12) {
        return `${months} ${months === 1 ? 'month' : 'months'} ago`;
    }
    const years = Math.floor(days / 365);
    return `${years} ${years === 1 ? 'year' : 'years'} ago`;
}
async function fetchApi(endpoint, options = {}) {
    const url = `${API_URL}${endpoint}`;
    const headers = getAuthHeaders();
    const response = await fetch(url, {
        ...options,
        headers: {
            ...headers,
            ...options.headers
        }
    });
    if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        throw new Error(error.message || `API request failed: ${response.status}`);
    }
    return response.json();
}
function showError(message) {
    console.error(message);
}
if (typeof module !== 'undefined') {
    module.exports = {
        API_URL,
        MEDIA_URL,
        WS_URL,
        getAuthToken,
        getAuthHeaders,
        isAuthenticated,
        formatDate,
        formatTimeAgo,
        fetchApi,
        showError
    };
}
