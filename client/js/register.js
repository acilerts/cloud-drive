const form = document.getElementById("registerForm");
const pwInput = document.getElementById("password");
const pwRepeatInput = document.getElementById("passwordRepeat");
const msgBox = document.getElementById("msgBox");
const submitBtn = document.getElementById("submitBtn");

const ruleLen = document.getElementById("rule-len");
const ruleUpper = document.getElementById("rule-upper");
const ruleLower = document.getElementById("rule-lower");
const ruleDigit = document.getElementById("rule-digit");
const ruleSpec = document.getElementById("rule-spec");

pwInput.addEventListener("input", () => {
    const val = pwInput.value;
    toggleRule(ruleLen, val.length >= 10);
    toggleRule(ruleUpper, /[A-Z]/.test(val));
    toggleRule(ruleLower, /[a-z]/.test(val));
    toggleRule(ruleDigit, /[0-9]/.test(val));
    toggleRule(ruleSpec, /[^A-Za-z0-9\s]/.test(val));
});

function toggleRule(el, ok) {
    if (ok) {
        el.classList.add("ok");
    } else {
        el.classList.remove("ok");
    }
}

function resetErrors() {
    msgBox.className = "msg-box";
    msgBox.textContent = "";
    document.querySelectorAll(".err-text").forEach(el => {
        el.textContent = "";
    });
    document.querySelectorAll("input").forEach(el => {
        el.classList.remove("invalid");
    });
}

function setFieldError(name, msg) {
    const errEl = document.getElementById("err-" + name);
    const inputEl = document.getElementById(name);
    if (errEl) errEl.textContent = msg;
    if (inputEl) inputEl.classList.add("invalid");
}

form.addEventListener("submit", async (e) => {
    e.preventDefault();
    resetErrors();

    const firstName = document.getElementById("firstName").value.trim();
    const lastName = document.getElementById("lastName").value.trim();
    const email = document.getElementById("email").value.trim();
    const password = pwInput.value;
    const passwordRepeat = pwRepeatInput.value;

    let hasErr = false;
    if (firstName.length < 1 || firstName.length > 50) {
        setFieldError("firstName", "Duzina mora biti od 1 do 50 karaktera");
        hasErr = true;
    }
    if (lastName.length < 1 || lastName.length > 50) {
        setFieldError("lastName", "Duzina mora biti od 1 do 50 karaktera");
        hasErr = true;
    }
    if (!email || !email.includes("@")) {
        setFieldError("email", "Neispravna imejl adresa");
        hasErr = true;
    }
    if (password.length < 10) {
        setFieldError("password", "Lozinka mora imati najmanje 10 karaktera");
        hasErr = true;
    }
    if (password !== passwordRepeat) {
        setFieldError("passwordRepeat", "Lozinke se ne poklapaju");
        hasErr = true;
    }

    if (hasErr) return;

    submitBtn.disabled = true;

    try {
        await sendRequest("/register", "POST", {
            firstName,
            lastName,
            email,
            password,
            passwordRepeat
        });

        msgBox.className = "msg-box success show";
        msgBox.textContent = "Registracija uspesna. Proverite imejl za potvrdu naloga.";
        form.reset();
        document.querySelectorAll(".pw-rules li").forEach(li => li.classList.remove("ok"));
    } catch (err) {
        if (err.status === 400 && err.data && err.data.fields) {
            for (const [k, v] of Object.entries(err.data.fields)) {
                setFieldError(k, v);
            }
        } else if (err.status === 409) {
            msgBox.className = "msg-box error show";
            msgBox.textContent = "Korisnik sa unetom imejl adresom vec postoji.";
        } else {
            msgBox.className = "msg-box error show";
            msgBox.textContent = err.message || "Doslo je do greske.";
        }
    } finally {
        submitBtn.disabled = false;
    }
});
