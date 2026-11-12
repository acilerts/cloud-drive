const API_BASE = "http://localhost:8080";

async function sendRequest(path, method = "GET", body = null) {
    const opts = {
        method: method,
        headers: {
            "Content-Type": "application/json"
        }
    };

    const token = localStorage.getItem("token");
    if (token) {
        opts.headers["Authorization"] = "Bearer " + token;
    }

    if (body) {
        opts.body = JSON.stringify(body);
    }

    const res = await fetch(API_BASE + path, opts);
    let data = {};
    try {
        data = await res.json();
    } catch (_) {}

    if (!res.ok) {
        const err = new Error(data.error || "Greska u zahtevu");
        err.status = res.status;
        err.data = data;
        throw err;
    }

    return data;
}
