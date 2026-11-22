const API_BASE = "";

async function parseResponse(res) {
    let data = {};
    try {
        data = await res.json();
    } catch (_) {}

    if (!res.ok) {
        const err = new Error(data.error || "Greska u zahtevu");
        err.status = res.status;
        err.code = data.code || "";
        err.data = data;
        err.fields = data.fields || null;
        throw err;
    }

    return data;
}

async function sendRequest(path, method = "GET", body = null) {
    const opts = {
        method,
        headers: {}
    };

    if (body !== null) {
        opts.headers["Content-Type"] = "application/json";
        opts.body = JSON.stringify(body);
    }

    const token = localStorage.getItem("token");
    if (token) {
        opts.headers["Authorization"] = "Bearer " + token;
    }

    const res = await fetch(API_BASE + path, opts);
    return parseResponse(res);
}

async function sendMultipart(path, formData, method = "POST") {
    const opts = {
        method,
        headers: {},
        body: formData
    };

    const token = localStorage.getItem("token");
    if (token) {
        opts.headers["Authorization"] = "Bearer " + token;
    }

    const res = await fetch(API_BASE + path, opts);
    return parseResponse(res);
}

function formatBytes(bytes) {
    const n = Number(bytes) || 0;
    if (n >= 1073741824) {
        return (n / 1073741824).toFixed(2) + " GB";
    }
    if (n >= 1048576) {
        return (n / 1048576).toFixed(1) + " MB";
    }
    if (n >= 1024) {
        return Math.round(n / 1024) + " KB";
    }
    return n + " B";
}

function formatDateTime(value) {
    if (!value) return "-";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return String(value);
    return d.toLocaleDateString("sr-RS") + " " + d.toLocaleTimeString("sr-RS", { hour: "2-digit", minute: "2-digit" });
}

function validateStrongPassword(password) {
    if (!password || password.length < 10) {
        return "Lozinka mora imati najmanje 10 karaktera.";
    }
    if (new TextEncoder().encode(password).length > 72) {
        return "Lozinka moze imati najvise 72 bajta.";
    }
    const ok = /[A-Z]/.test(password) &&
        /[a-z]/.test(password) &&
        /[0-9]/.test(password) &&
        /[^A-Za-z0-9\s]/.test(password);
    if (!ok) {
        return "Lozinka mora sadrzati veliko slovo, malo slovo, cifru i specijalni karakter.";
    }
    return "";
}
