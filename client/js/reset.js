const form = document.getElementById("resetPwForm");
const pwInput = document.getElementById("password");
const pwRepeatInput = document.getElementById("passwordRepeat");
const msgBox = document.getElementById("msgBox");
const submitBtn = document.getElementById("submitBtn");

const params = new URLSearchParams(window.location.search);
const token = params.get("token");

if (!token) {
    msgBox.className = "msg-box error show";
    msgBox.textContent = "Token za reset lozinke nije pronadjen u adresi.";
    submitBtn.disabled = true;
}

const ruleLen = document.getElementById("rule-len");
const ruleUpper = document.getElementById("rule-upper");
const ruleLower = document.getElementById("rule-lower");
const ruleDigit = document.getElementById("rule-digit");
const ruleSpec = document.getElementById("rule-spec");

function toggleRule(el, ok) {
    if (ok) el.classList.add("ok");
    else el.classList.remove("ok");
}

pwInput.addEventListener("input", () => {
    const val = pwInput.value;
    toggleRule(ruleLen, val.length >= 10);
    toggleRule(ruleUpper, /[A-Z]/.test(val));
    toggleRule(ruleLower, /[a-z]/.test(val));
    toggleRule(ruleDigit, /[0-9]/.test(val));
    toggleRule(ruleSpec, /[^A-Za-z0-9\s]/.test(val));
});

function clearErrors() {
    msgBox.className = "msg-box";
    msgBox.textContent = "";
    document.querySelectorAll(".err-text").forEach(el => { el.textContent = ""; });
    document.querySelectorAll("input").forEach(el => { el.classList.remove("invalid"); });
}

function setFieldError(name, msg) {
    const errEl = document.getElementById("err-" + name);
    const inputEl = document.getElementById(name);
    if (errEl) errEl.textContent = msg;
    if (inputEl) inputEl.classList.add("invalid");
}

form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (!token) return;
    clearErrors();

    const password = pwInput.value;
    const passwordRepeat = pwRepeatInput.value;

    let hasErr = false;
    const pwErr = validateStrongPassword(password);
    if (pwErr) {
        setFieldError("password", pwErr);
        hasErr = true;
    }
    if (password !== passwordRepeat) {
        setFieldError("passwordRepeat", "Lozinke se ne poklapaju.");
        hasErr = true;
    }
    if (hasErr) return;

    submitBtn.disabled = true;

    try {
        await sendRequest("/password/reset", "POST", { token, password, passwordRepeat });
        msgBox.className = "msg-box success show";
        msgBox.textContent = "Lozinka je uspesno promenjena. Mozete se prijaviti novom lozinkom.";
        form.reset();
        document.querySelectorAll(".pw-rules li").forEach(li => li.classList.remove("ok"));
    } catch (err) {
        if (err.status === 400 && err.fields) {
            for (const [k, v] of Object.entries(err.fields)) {
                setFieldError(k, v);
            }
        } else {
            msgBox.className = "msg-box error show";
            msgBox.textContent = err.message || "Token nije ispravan, istekao je ili je vec iskoriscen.";
        }
    } finally {
        submitBtn.disabled = false;
    }
});
