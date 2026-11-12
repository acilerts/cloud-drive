const statusText = document.getElementById("statusText");
const msgBox = document.getElementById("msgBox");
const loginLinkWrap = document.getElementById("loginLinkWrap");

async function confirmAccount() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get("token");

    if (!token) {
        statusText.textContent = "Neispravan link";
        msgBox.className = "msg-box error show";
        msgBox.textContent = "Token za potvrdu nije pronadjen u adresi.";
        return;
    }

    try {
        await sendRequest("/confirm", "POST", { token });
        statusText.textContent = "Nalog je potvrdjen";
        msgBox.className = "msg-box success show";
        msgBox.textContent = "Uspesno ste aktivirali nalog. Mozete se prijaviti.";
        loginLinkWrap.style.display = "block";
    } catch (err) {
        statusText.textContent = "Potvrda nije uspela";
        msgBox.className = "msg-box error show";
        msgBox.textContent = "Token nije ispravan, istekao je ili je vec iskoriscen.";
    }
}

confirmAccount();
