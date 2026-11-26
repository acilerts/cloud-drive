const credentialsStep = document.getElementById("credentialsStep");
const otpStep = document.getElementById("otpStep");
const resetStep = document.getElementById("resetStep");
const magicStep = document.getElementById("magicStep");

const loginForm = document.getElementById("loginForm");
const otpForm = document.getElementById("otpForm");
const resetForm = document.getElementById("resetForm");
const magicForm = document.getElementById("magicForm");

const loginMsg = document.getElementById("loginMsg");
const otpMsg = document.getElementById("otpMsg");
const resetMsg = document.getElementById("resetMsg");
const magicMsg = document.getElementById("magicMsg");

const loginBtn = document.getElementById("loginBtn");
const otpBtn = document.getElementById("otpBtn");
const resetBtn = document.getElementById("resetBtn");
const magicBtn = document.getElementById("magicBtn");

const otpInputs = document.querySelectorAll(".otp-input");

let currentEmail = "";

function showStep(stepEl) {
    [credentialsStep, otpStep, resetStep, magicStep].forEach(el => {
        el.style.display = el === stepEl ? "block" : "none";
    });
}

function mapLoginError(err) {
    if (err.status === 429) {
        if (err.code === "account_locked") {
            return "Previse neuspesnih pokusaja. Nalog je privremeno zakljucan.";
        }
        return "Previse zahteva. Pokusajte ponovo za par trenutaka.";
    }
    if (err.code === "account_not_confirmed") {
        return "Nalog nije potvrdjen. Proverite imejl za link za aktivaciju.";
    }
    if (err.code === "password_expired") {
        return "Vasa lozinka je istekla. Iskoristite opciju za reset lozinke.";
    }
    return err.message || "Pogresan imejl ili lozinka.";
}

otpInputs.forEach((input, idx) => {
    input.addEventListener("input", () => {
        input.value = input.value.replace(/\D/g, "").slice(0, 1);
        if (input.value && idx < otpInputs.length - 1) {
            otpInputs[idx + 1].focus();
        }
    });
    input.addEventListener("keydown", (e) => {
        if (e.key === "Backspace" && !input.value && idx > 0) {
            otpInputs[idx - 1].focus();
        }
    });
    input.addEventListener("paste", (e) => {
        const text = (e.clipboardData || window.clipboardData).getData("text").replace(/\D/g, "");
        if (text.length === otpInputs.length) {
            e.preventDefault();
            otpInputs.forEach((inp, i) => { inp.value = text[i] || ""; });
            otpInputs[otpInputs.length - 1].focus();
        }
    });
});

loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    loginMsg.className = "msg-box";
    loginMsg.textContent = "";

    currentEmail = document.getElementById("email").value.trim().toLowerCase();
    const password = document.getElementById("password").value;

    loginBtn.disabled = true;

    try {
        await sendRequest("/login", "POST", { email: currentEmail, password });
        showStep(otpStep);
        otpInputs.forEach(i => { i.value = ""; });
        otpInputs[0].focus();
    } catch (err) {
        loginMsg.className = "msg-box error show";
        loginMsg.textContent = mapLoginError(err);
    } finally {
        loginBtn.disabled = false;
    }
});

otpForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    otpMsg.className = "msg-box";
    otpMsg.textContent = "";

    let code = "";
    otpInputs.forEach(i => { code += i.value; });

    if (code.length < 6) {
        otpMsg.className = "msg-box error show";
        otpMsg.textContent = "Unesite sestocifreni kod.";
        return;
    }

    otpBtn.disabled = true;

    try {
        const res = await sendRequest("/login/verify", "POST", { email: currentEmail, otp: code });
        if (res.token) {
            localStorage.setItem("token", res.token);
            localStorage.removeItem("user");
        }
        if (res.user && res.user.role === "admin") {
            window.location.href = "admin.html";
        } else {
            window.location.href = "index.html";
        }
    } catch (err) {
        otpMsg.className = "msg-box error show";
        otpMsg.textContent = err.status === 429
            ? "Previse zahteva. Pokusajte ponovo za par trenutaka."
            : (err.message || "Neispravan ili istekao kod.");
    } finally {
        otpBtn.disabled = false;
    }
});

document.getElementById("backBtn").addEventListener("click", (e) => {
    e.preventDefault();
    showStep(credentialsStep);
});

document.getElementById("forgotPwLink").addEventListener("click", (e) => {
    e.preventDefault();
    resetMsg.className = "msg-box";
    resetMsg.textContent = "";
    showStep(resetStep);
});

document.getElementById("backFromResetBtn").addEventListener("click", (e) => {
    e.preventDefault();
    showStep(credentialsStep);
});

document.getElementById("magicLinkOpen").addEventListener("click", (e) => {
    e.preventDefault();
    magicMsg.className = "msg-box";
    magicMsg.textContent = "";
    showStep(magicStep);
});

document.getElementById("backFromMagicBtn").addEventListener("click", (e) => {
    e.preventDefault();
    showStep(credentialsStep);
});

resetForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    resetMsg.className = "msg-box";
    const email = document.getElementById("resetEmail").value.trim().toLowerCase();
    if (!email) return;

    resetBtn.disabled = true;
    try {
        await sendRequest("/password/forgot", "POST", { email });
        resetMsg.className = "msg-box success show";
        resetMsg.textContent = "Ako nalog sa unetom adresom postoji, link za reset lozinke je poslat na imejl.";
    } catch (err) {
        resetMsg.className = "msg-box error show";
        resetMsg.textContent = err.status === 429
            ? "Previse zahteva. Pokusajte ponovo kasnije."
            : (err.message || "Slanje linka nije uspelo.");
    } finally {
        resetBtn.disabled = false;
    }
});

magicForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    magicMsg.className = "msg-box";
    const email = document.getElementById("magicEmail").value.trim().toLowerCase();
    if (!email) return;

    magicBtn.disabled = true;
    try {
        await sendRequest("/magic/request", "POST", { email });
        magicMsg.className = "msg-box success show";
        magicMsg.textContent = "Ako nalog postoji, magicni link za prijavu je poslat na imejl.";
    } catch (err) {
        magicMsg.className = "msg-box error show";
        magicMsg.textContent = err.status === 429
            ? "Previse zahteva. Pokusajte ponovo kasnije."
            : (err.message || "Slanje magicnog linka nije uspelo.");
    } finally {
        magicBtn.disabled = false;
    }
});
