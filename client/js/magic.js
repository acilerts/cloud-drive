const statusText = document.getElementById("statusText");
const msgBox = document.getElementById("msgBox");
const loginLinkWrap = document.getElementById("loginLinkWrap");

async function loginWithMagicLink() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get("token");

    if (!token) {
        statusText.textContent = "Neispravan link";
        msgBox.className = "msg-box error show";
        msgBox.textContent = "Token za prijavu nije pronadjen u adresi.";
        loginLinkWrap.style.display = "block";
        return;
    }

    try {
        const res = await sendRequest("/magic/login", "POST", { token });
        if (res.token) {
            localStorage.setItem("token", res.token);
        }
        if (res.user) {
            localStorage.setItem("user", JSON.stringify(res.user));
        }

        statusText.textContent = "Prijava uspesna";
        msgBox.className = "msg-box success show";
        msgBox.textContent = "Preusmeravanje na aplikaciju...";

        setTimeout(() => {
            if (res.user && res.user.role === "admin") {
                window.location.href = "admin.html";
            } else {
                window.location.href = "index.html";
            }
        }, 600);
    } catch (err) {
        statusText.textContent = "Prijava nije uspela";
        msgBox.className = "msg-box error show";
        if (err.code === "password_expired") {
            msgBox.textContent = "Lozinka je istekla. Potrebno je resetovati lozinku putem imejla.";
        } else {
            msgBox.textContent = err.message || "Magicni link nije ispravan, istekao je ili je vec iskoriscen.";
        }
        loginLinkWrap.style.display = "block";
    }
}

loginWithMagicLink();
