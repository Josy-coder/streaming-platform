
const API_URL = 'http://localhost:3000';
let authToken = null;
let currentUser = null;
function checkAuth() {
    const token = localStorage.getItem('token');
    if (!token) {
        return Promise.resolve(false);
    }
    return fetch(`${API_URL}/api/user`, {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
        .then(response => {
            if (!response.ok) {
                localStorage.removeItem('token');
                return false;
            }
            return response.json().then(userData => {
                authToken = token;
                currentUser = userData;
                return true;
            });
        })
        .catch(error => {
            console.error('Auth check error:', error);
            localStorage.removeItem('token');
            return false;
        });
}
function login(username, password) {
    return fetch(`${API_URL}/api/auth/login`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            username,
            password
        })
    })
        .then(response => {
            if (!response.ok) {
                return response.json().then(error => {
                    throw new Error(error.error || 'Login failed');
                });
            }
            return response.json();
        })
        .then(data => {
            authToken = data.token;
            currentUser = data.user;
            localStorage.setItem('token', data.token);
            return data.user;
        });
}
function register(username, email, password) {
    return fetch(`${API_URL}/api/auth/register`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            username,
            email,
            password
        })
    })
        .then(response => {
            if (!response.ok) {
                return response.json().then(error => {
                    throw new Error(error.error || 'Registration failed');
                });
            }
            return response.json();
        })
        .then(data => {
            authToken = data.token;
            currentUser = data.user;
            localStorage.setItem('token', data.token);
            return data.user;
        });
}
function logout() {
    authToken = null;
    currentUser = null;
    localStorage.removeItem('token');
}
function getCurrentUser() {
    if (!authToken) {
        return Promise.reject(new Error('Not authenticated'));
    }
    return fetch(`${API_URL}/api/user`, {
        headers: {
            'Authorization': `Bearer ${authToken}`
        }
    })
        .then(response => {
            if (!response.ok) {
                logout();
                throw new Error('Invalid token');
            }
            return response.json();
        })
        .then(userData => {
            currentUser = userData;
            return userData;
        });
}
function updateProfile(data) {
    if (!authToken) {
        return Promise.reject(new Error('Not authenticated'));
    }
    return fetch(`${API_URL}/api/user`, {
        method: 'PUT',
        headers: {
            'Authorization': `Bearer ${authToken}`,
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    })
        .then(response => {
            if (!response.ok) {
                return response.json().then(error => {
                    throw new Error(error.error || 'Update failed');
                });
            }
            return response.json();
        })
        .then(userData => {
            currentUser = userData;
            return userData;
        });
}
function getAuthHeaders() {
    if (!authToken) {
        return {};
    }
    return {
        'Authorization': `Bearer ${authToken}`
    };
}
function isAuthenticated() {
    return !!authToken;
}
if (typeof module !== 'undefined') {
    module.exports = {
        checkAuth,
        login,
        register,
        logout,
        getCurrentUser,
        updateProfile,
        getAuthHeaders,
        isAuthenticated
    };
}
