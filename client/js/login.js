const credentialsStep = document.getElementById("credentialsStep");
const otpStep = document.getElementById("otpStep");
const loginForm = document.getElementById("loginForm");
const otpForm = document.getElementById("otpForm");
const loginMsg = document.getElementById("loginMsg");
const otpMsg = document.getElementById("otpMsg");
const loginBtn = document.getElementById("loginBtn");
const otpBtn = document.getElementById("otpBtn");
const backBtn = document.getElementById("backBtn");
const otpInputs = document.querySelectorAll(".otp-input");

let currentEmail = "";

otpInputs.forEach((input, idx) => {
    input.addEventListener("input", () => {
        if (input.value && idx < otpInputs.length - 1) {
            otpInputs[idx + 1].focus();
        }
    });
    input.addEventListener("keydown", (e) => {
        if (e.key === "Backspace" && !input.value && idx > 0) {
            otpInputs[idx - 1].focus();
        }
    });
});

loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    loginMsg.className = "msg-box";
    loginMsg.textContent = "";

    currentEmail = document.getElementById("email").value.trim();
    const password = document.getElementById("password").value;

    loginBtn.disabled = true;

    try {
        await sendRequest("/login", "POST", { email: currentEmail, password });
        credentialsStep.style.display = "none";
        otpStep.style.display = "block";
        otpInputs.forEach(i => { i.value = ""; });
        otpInputs[0].focus();
    } catch (err) {
        loginMsg.className = "msg-box error show";
        loginMsg.textContent = err.message || "Pogresan imejl ili lozinka.";
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
        }
        if (res.user) {
            localStorage.setItem("user", JSON.stringify(res.user));
        }
        window.location.href = "index.html";
    } catch (err) {
        otpMsg.className = "msg-box error show";
        otpMsg.textContent = err.message || "Neispravan ili istekao kod.";
    } finally {
        otpBtn.disabled = false;
    }
});

backBtn.addEventListener("click", (e) => {
    e.preventDefault();
    otpStep.style.display = "none";
    credentialsStep.style.display = "block";
});

const resetStep = document.getElementById("resetStep");
const resetForm = document.getElementById("resetForm");
const resetMsg = document.getElementById("resetMsg");
const forgotPwLink = document.getElementById("forgotPwLink");
const backFromResetBtn = document.getElementById("backFromResetBtn");

forgotPwLink.addEventListener("click", (e) => {
    e.preventDefault();
    credentialsStep.style.display = "none";
    resetStep.style.display = "block";
});

backFromResetBtn.addEventListener("click", (e) => {
    e.preventDefault();
    resetStep.style.display = "none";
    credentialsStep.style.display = "block";
});

resetForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    resetMsg.className = "msg-box";
    const email = document.getElementById("resetEmail").value.trim();
    if (!email) return;

    try {
        await sendRequest("/password-reset/request", "POST", { email });
        resetMsg.className = "msg-box success show";
        resetMsg.textContent = "Link za promenu lozinke je poslat na imejl.";
    } catch (err) {
        resetMsg.className = "msg-box error show";
        resetMsg.textContent = err.message || "Slanje linka nije uspelo.";
    }
});
