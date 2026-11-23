const token = localStorage.getItem("token");
const savedUser = JSON.parse(localStorage.getItem("user") || "null");

if (!token || !savedUser) {
    window.location.href = "login.html";
} else if (savedUser.role !== "admin") {
    window.location.href = "index.html";
}

if (savedUser && savedUser.firstName) {
    document.getElementById("userName").textContent = savedUser.firstName + " " + (savedUser.lastName || "");
    document.getElementById("userAvatar").textContent = savedUser.firstName[0].toUpperCase();
}

const adminMsg = document.getElementById("adminMsg");
const requestsBody = document.getElementById("requestsBody");
const usersBody = document.getElementById("usersBody");
const changePwModal = document.getElementById("changePwModal");

let currentStatusFilter = "pending";

function showBanner(text, type = "error") {
    adminMsg.className = "msg-box " + type + " show";
    adminMsg.textContent = text;
}

function clearBanner() {
    adminMsg.className = "msg-box";
    adminMsg.textContent = "";
}

function makeStatusBadge(status) {
    const span = document.createElement("span");
    span.className = "status-pill status-" + status;
    const labels = {
        pending: "Na cekanju",
        approved: "Odobreno",
        rejected: "Odbijeno",
        active: "Aktivan"
    };
    span.textContent = labels[status] || status;
    return span;
}

async function loadQuotaRequests(status = currentStatusFilter) {
    currentStatusFilter = status;
    clearBanner();
    requestsBody.innerHTML = "";

    try {
        const res = await sendRequest("/admin/quota-requests?status=" + encodeURIComponent(status), "GET");
        const list = res.requests || [];

        if (list.length === 0) {
            const tr = document.createElement("tr");
            const td = document.createElement("td");
            td.colSpan = 6;
            td.className = "empty-cell";
            td.textContent = "Nema zahteva sa izabranim statusom.";
            tr.appendChild(td);
            requestsBody.appendChild(tr);
            return;
        }

        list.forEach(qr => {
            const tr = document.createElement("tr");

            const tdEmail = document.createElement("td");
            tdEmail.textContent = qr.email || qr.userId;

            const tdCurr = document.createElement("td");
            tdCurr.textContent = qr.currentQuotaBytes != null ? formatBytes(qr.currentQuotaBytes) : "-";

            const tdReq = document.createElement("td");
            tdReq.textContent = formatBytes(qr.requestedBytes);

            const tdStatus = document.createElement("td");
            tdStatus.appendChild(makeStatusBadge(qr.status));

            const tdDate = document.createElement("td");
            tdDate.textContent = formatDateTime(qr.createdAt);

            const tdActions = document.createElement("td");
            const actionsWrap = document.createElement("div");
            actionsWrap.className = "row-actions";

            if (qr.status === "pending") {
                const approveBtn = document.createElement("button");
                approveBtn.className = "btn btn-primary btn-sm";
                approveBtn.style.width = "auto";
                approveBtn.textContent = "Odobri";
                approveBtn.addEventListener("click", () => decideRequest(qr.id, true, approveBtn));

                const rejectBtn = document.createElement("button");
                rejectBtn.className = "btn btn-danger btn-sm";
                rejectBtn.textContent = "Odbij";
                rejectBtn.addEventListener("click", () => decideRequest(qr.id, false, rejectBtn));

                actionsWrap.appendChild(approveBtn);
                actionsWrap.appendChild(rejectBtn);
            } else {
                const span = document.createElement("span");
                span.style.color = "#64748b";
                span.style.fontSize = "12px";
                span.textContent = qr.decidedAt ? formatDateTime(qr.decidedAt) : "Odluceno";
                actionsWrap.appendChild(span);
            }

            tdActions.appendChild(actionsWrap);

            tr.appendChild(tdEmail);
            tr.appendChild(tdCurr);
            tr.appendChild(tdReq);
            tr.appendChild(tdStatus);
            tr.appendChild(tdDate);
            tr.appendChild(tdActions);
            requestsBody.appendChild(tr);
        });
    } catch (err) {
        if (err.status === 401 || err.status === 403) {
            localStorage.removeItem("token");
            localStorage.removeItem("user");
            window.location.href = "login.html";
            return;
        }
        showBanner(err.message || "Ucitavanje zahteva nije uspelo.");
    }
}

async function decideRequest(id, approve, btn) {
    clearBanner();
    btn.disabled = true;
    const action = approve ? "approve" : "reject";
    try {
        await sendRequest("/admin/quota-requests/" + encodeURIComponent(id) + "/" + action, "POST");
        showBanner(approve ? "Zahtev je odobren i kvota korisnika je azurirana." : "Zahtev je odbijen.", "success");
        await loadQuotaRequests(currentStatusFilter);
    } catch (err) {
        showBanner(err.message || "Akcija nije uspela.");
        btn.disabled = false;
    }
}

async function loadUsers() {
    clearBanner();
    usersBody.innerHTML = "";

    try {
        const res = await sendRequest("/admin/users", "GET");
        const users = res.users || [];

        if (users.length === 0) {
            const tr = document.createElement("tr");
            const td = document.createElement("td");
            td.colSpan = 6;
            td.className = "empty-cell";
            td.textContent = "Nema registrovanih korisnika.";
            tr.appendChild(td);
            usersBody.appendChild(tr);
            return;
        }

        users.forEach(u => {
            const tr = document.createElement("tr");

            const tdName = document.createElement("td");
            tdName.textContent = (u.firstName || "") + " " + (u.lastName || "");

            const tdEmail = document.createElement("td");
            tdEmail.textContent = u.email;

            const tdRole = document.createElement("td");
            tdRole.textContent = u.role === "admin" ? "Administrator" : "Redovan korisnik";

            const tdStatus = document.createElement("td");
            tdStatus.appendChild(makeStatusBadge(u.status));

            const tdQuota = document.createElement("td");
            tdQuota.textContent = formatBytes(u.quotaBytes);

            const tdUsed = document.createElement("td");
            tdUsed.textContent = u.usedBytes != null ? formatBytes(u.usedBytes) : "0 B";

            tr.appendChild(tdName);
            tr.appendChild(tdEmail);
            tr.appendChild(tdRole);
            tr.appendChild(tdStatus);
            tr.appendChild(tdQuota);
            tr.appendChild(tdUsed);
            usersBody.appendChild(tr);
        });
    } catch (err) {
        if (err.status === 401 || err.status === 403) {
            localStorage.removeItem("token");
            localStorage.removeItem("user");
            window.location.href = "login.html";
            return;
        }
        showBanner(err.message || "Ucitavanje korisnika nije uspelo.");
    }
}

document.querySelectorAll(".menu li").forEach(li => {
    li.addEventListener("click", () => {
        document.querySelectorAll(".menu li").forEach(el => el.classList.remove("active"));
        li.classList.add("active");
        const sec = li.dataset.section;
        document.getElementById("view-requests").style.display = sec === "requests" ? "block" : "none";
        document.getElementById("view-users").style.display = sec === "users" ? "block" : "none";
        if (sec === "users") {
            loadUsers();
        } else {
            loadQuotaRequests(currentStatusFilter);
        }
    });
});

document.querySelectorAll(".filter-btn").forEach(btn => {
    btn.addEventListener("click", () => {
        document.querySelectorAll(".filter-btn").forEach(b => b.classList.remove("active"));
        btn.classList.add("active");
        loadQuotaRequests(btn.dataset.status);
    });
});

document.getElementById("refreshUsersBtn").addEventListener("click", loadUsers);

document.getElementById("logoutBtn").addEventListener("click", () => {
    localStorage.removeItem("token");
    localStorage.removeItem("user");
    window.location.href = "login.html";
});

document.getElementById("openChangePwBtn").addEventListener("click", () => {
    document.getElementById("oldPwInput").value = "";
    document.getElementById("newPwInput").value = "";
    document.getElementById("repeatNewPwInput").value = "";
    document.getElementById("changePwMsg").className = "msg-box";
    changePwModal.classList.add("open");
});

document.querySelectorAll(".close-modal").forEach(btn => {
    btn.addEventListener("click", () => changePwModal.classList.remove("open"));
});

changePwModal.addEventListener("click", (e) => {
    if (e.target === changePwModal) changePwModal.classList.remove("open");
});

document.getElementById("saveChangePwBtn").addEventListener("click", async () => {
    const currentPassword = document.getElementById("oldPwInput").value;
    const newPassword = document.getElementById("newPwInput").value;
    const newPasswordRepeat = document.getElementById("repeatNewPwInput").value;
    const msg = document.getElementById("changePwMsg");

    if (!currentPassword) {
        msg.className = "msg-box error show";
        msg.textContent = "Unesite trenutnu lozinku.";
        return;
    }
    const pwErr = validateStrongPassword(newPassword);
    if (pwErr) {
        msg.className = "msg-box error show";
        msg.textContent = pwErr;
        return;
    }
    if (newPassword !== newPasswordRepeat) {
        msg.className = "msg-box error show";
        msg.textContent = "Lozinke se ne poklapaju.";
        return;
    }

    try {
        await sendRequest("/password/change", "POST", { currentPassword, newPassword, newPasswordRepeat });
        msg.className = "msg-box success show";
        msg.textContent = "Lozinka je uspesno promenjena.";
    } catch (err) {
        msg.className = "msg-box error show";
        if (err.code === "password_too_new") {
            msg.textContent = "Lozinka mora biti stara bar 1 dan pre nove promene.";
        } else if (err.code === "wrong_password") {
            msg.textContent = "Trenutna lozinka nije ispravna.";
        } else if (err.fields && err.fields.newPassword) {
            msg.textContent = err.fields.newPassword;
        } else {
            msg.textContent = err.message || "Promena lozinke nije uspela.";
        }
    }
});

loadQuotaRequests("pending");
