const token = localStorage.getItem("token");
const savedUser = JSON.parse(localStorage.getItem("user") || "null");

if (!token) {
    window.location.href = "login.html";
}

if (savedUser && savedUser.firstName) {
    document.getElementById("userName").textContent = savedUser.firstName + " " + (savedUser.lastName || "");
    document.getElementById("userAvatar").textContent = savedUser.firstName[0].toUpperCase();
}
if (savedUser && savedUser.role === "admin") {
    const adminLink = document.getElementById("adminPanelLink");
    if (adminLink) adminLink.style.display = "inline-block";
}

const folderModal = document.getElementById("folderModal");
const uploadModal = document.getElementById("uploadModal");
const renameModal = document.getElementById("renameModal");
const quotaModal = document.getElementById("quotaModal");
const previewModal = document.getElementById("previewModal");
const changePwModal = document.getElementById("changePwModal");
const notifModal = document.getElementById("notifModal");

const driveMsg = document.getElementById("driveMsg");
const foldersList = document.getElementById("foldersList");
const filesBody = document.getElementById("filesBody");
const activityBody = document.getElementById("activityBody");
const breadcrumbsEl = document.getElementById("breadcrumbs");
const searchInput = document.getElementById("searchInput");
const renameInput = document.getElementById("renameInput");

const MAX_UPLOAD_BYTES = 50 * 1024 * 1024;

let pathStack = [{ id: null, name: "Moji fajlovi" }];
let currentFolders = [];
let currentFiles = [];
let renameTarget = null;
let versionTargetFile = null;

let quotaBytes = 1073741824;
let usedBytes = 0;
let myQuotaRequests = [];
const activities = [];

function currentFolderId() {
    return pathStack[pathStack.length - 1].id;
}

function showDriveMsg(text, type = "error") {
    driveMsg.className = "msg-box " + type + " show";
    driveMsg.textContent = text;
}

function clearDriveMsg() {
    driveMsg.className = "msg-box";
    driveMsg.textContent = "";
}

function showModalMsg(id, text, type = "error") {
    const el = document.getElementById(id);
    if (!el) return;
    el.className = "msg-box " + type + " show";
    el.textContent = text;
}

function clearModalMsg(id) {
    const el = document.getElementById(id);
    if (!el) return;
    el.className = "msg-box";
    el.textContent = "";
}

function handleAuthError(err) {
    if (err && err.status === 401) {
        localStorage.removeItem("token");
        localStorage.removeItem("user");
        window.location.href = "login.html";
        return true;
    }
    return false;
}

function logActivity(action, item) {
    activities.unshift({ action, item, time: formatDateTime(new Date().toISOString()) });
    renderActivities();
}

function renderActivities() {
    activityBody.innerHTML = "";
    if (activities.length === 0) {
        const tr = document.createElement("tr");
        const td = document.createElement("td");
        td.colSpan = 3;
        td.className = "empty-cell";
        td.textContent = "Nema zabelezenih aktivnosti u ovoj sesiji.";
        tr.appendChild(td);
        activityBody.appendChild(tr);
        return;
    }
    activities.forEach(a => {
        const tr = document.createElement("tr");
        const tdAct = document.createElement("td");
        tdAct.textContent = a.action;
        const tdItem = document.createElement("td");
        tdItem.textContent = a.item;
        const tdTime = document.createElement("td");
        tdTime.textContent = a.time;
        tr.appendChild(tdAct);
        tr.appendChild(tdItem);
        tr.appendChild(tdTime);
        activityBody.appendChild(tr);
    });
}

function updateQuotaUI() {
    const pct = quotaBytes > 0 ? Math.min(100, Math.round((usedBytes / quotaBytes) * 100)) : 0;
    document.getElementById("quotaLabel").textContent = formatBytes(usedBytes) + " / " + formatBytes(quotaBytes);
    document.getElementById("quotaFill").style.width = Math.max(2, pct) + "%";
    const modalQuota = document.getElementById("currentQuotaModal");
    if (modalQuota) modalQuota.textContent = formatBytes(quotaBytes);
}

function renderQuotaRequests() {
    const ul = document.getElementById("myQuotaRequestsList");
    if (!ul) return;
    ul.innerHTML = "";
    if (!myQuotaRequests.length) {
        const li = document.createElement("li");
        li.style.color = "#64748b";
        li.textContent = "Nemate poslatih zahteva.";
        ul.appendChild(li);
        return;
    }
    const labels = { pending: "Na cekanju", approved: "Odobreno", rejected: "Odbijeno" };
    myQuotaRequests.forEach(qr => {
        const li = document.createElement("li");
        const title = document.createElement("div");
        title.textContent = "Trazeno: " + formatBytes(qr.requestedBytes) + " — " + (labels[qr.status] || qr.status);
        const span = document.createElement("span");
        span.className = "notif-time";
        span.textContent = formatDateTime(qr.createdAt);
        li.appendChild(title);
        li.appendChild(span);
        ul.appendChild(li);
    });
}

async function loadQuota() {
    try {
        const res = await sendRequest("/quota/requests/mine", "GET");
        if (typeof res.quotaBytes === "number") {
            quotaBytes = res.quotaBytes;
        }
        myQuotaRequests = Array.isArray(res.requests) ? res.requests : [];
        updateQuotaUI();
        renderQuotaRequests();
    } catch (err) {
        if (!handleAuthError(err)) {
            updateQuotaUI();
        }
    }
}

function renderBreadcrumbs() {
    breadcrumbsEl.innerHTML = "";
    pathStack.forEach((entry, idx) => {
        if (idx > 0) {
            breadcrumbsEl.appendChild(document.createTextNode(" / "));
        }
        const span = document.createElement("span");
        span.textContent = entry.name;
        span.dataset.index = String(idx);
        if (idx === pathStack.length - 1) {
            span.className = "current";
        }
        breadcrumbsEl.appendChild(span);
    });
}

function renderDrive(filterQuery = "") {
    const q = filterQuery.toLowerCase().trim();
    renderBreadcrumbs();

    foldersList.innerHTML = "";
    const visibleFolders = currentFolders.filter(f => !q || (f.name || "").toLowerCase().includes(q));

    if (visibleFolders.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-muted";
        empty.textContent = q ? "Nema foldera koji odgovaraju pretrazi." : "Nema podfoldera u ovom folderu.";
        foldersList.appendChild(empty);
    } else {
        visibleFolders.forEach(f => {
            const div = document.createElement("div");
            div.className = "folder-item";
            div.dataset.id = f.id;

            const span = document.createElement("span");
            span.className = "folder-label";
            span.textContent = f.name;

            const btn = document.createElement("button");
            btn.className = "btn btn-outline btn-sm rename-folder-btn";
            btn.textContent = "Preimenuj";

            div.appendChild(span);
            div.appendChild(btn);
            foldersList.appendChild(div);
        });
    }

    filesBody.innerHTML = "";
    const visibleFiles = currentFiles.filter(f => !q || (f.name || "").toLowerCase().includes(q));

    if (visibleFiles.length === 0) {
        const tr = document.createElement("tr");
        const td = document.createElement("td");
        td.colSpan = 5;
        td.className = "empty-cell";
        td.textContent = q ? "Nema datoteka koje odgovaraju pretrazi." : "Nema datoteka u ovom folderu.";
        tr.appendChild(td);
        filesBody.appendChild(tr);
    } else {
        visibleFiles.forEach(f => {
            const tr = document.createElement("tr");
            tr.dataset.id = f.id;

            const tdName = document.createElement("td");
            tdName.className = "file-title";
            tdName.textContent = f.name;

            const tdVer = document.createElement("td");
            tdVer.textContent = "v" + (f.currentVersion || f.version || 1);

            const tdDate = document.createElement("td");
            tdDate.textContent = formatDateTime(f.updatedAt || f.createdAt);

            const tdSize = document.createElement("td");
            tdSize.textContent = f.sizeBytes != null ? formatBytes(f.sizeBytes) : "-";

            const tdActions = document.createElement("td");
            const actionsWrap = document.createElement("div");
            actionsWrap.className = "row-actions";

            const versionBtn = document.createElement("button");
            versionBtn.className = "btn btn-outline btn-sm version-file-btn";
            versionBtn.textContent = "Nova verzija";

            const renameBtn = document.createElement("button");
            renameBtn.className = "btn btn-outline btn-sm rename-file-btn";
            renameBtn.textContent = "Preimenuj";

            actionsWrap.appendChild(versionBtn);
            actionsWrap.appendChild(renameBtn);
            tdActions.appendChild(actionsWrap);

            tr.appendChild(tdName);
            tr.appendChild(tdVer);
            tr.appendChild(tdDate);
            tr.appendChild(tdSize);
            tr.appendChild(tdActions);
            filesBody.appendChild(tr);
        });
    }

    usedBytes = currentFiles.reduce((sum, f) => sum + (Number(f.sizeBytes) || 0), 0);
    updateQuotaUI();
}

async function loadFolderContents() {
    clearDriveMsg();
    const pid = currentFolderId();
    const query = pid ? "?parentId=" + encodeURIComponent(pid) : "";

    try {
        const res = await sendRequest("/folders/contents" + query, "GET");
        currentFolders = Array.isArray(res.folders) ? res.folders : [];
        currentFiles = Array.isArray(res.files) ? res.files : [];
    } catch (err) {
        if (handleAuthError(err)) return;
        currentFolders = [];
        currentFiles = [];
        if (err.status === 502) {
            showDriveMsg("Files servis trenutno nije dostupan.");
        } else {
            showDriveMsg(err.message || "Ucitavanje sadrzaja foldera nije uspelo.");
        }
    }

    renderDrive(searchInput.value);
}

async function loadNotifications(silent = false) {
    const ul = document.getElementById("notifList");
    const badge = document.getElementById("notifBadge");
    if (!silent) {
        clearModalMsg("notifMsg");
        ul.innerHTML = "";
    }

    try {
        const res = await sendRequest("/notifications", "GET");
        const list = Array.isArray(res) ? res : (res.notifications || []);

        if (badge) {
            if (list.length > 0 && silent) {
                badge.textContent = String(list.length);
                badge.style.display = "inline-block";
            } else if (!silent) {
                badge.style.display = "none";
            }
        }

        if (silent) return;

        if (list.length === 0) {
            const li = document.createElement("li");
            li.style.color = "#64748b";
            li.textContent = "Nemate novih notifikacija.";
            ul.appendChild(li);
            return;
        }

        list.forEach(n => {
            const li = document.createElement("li");
            const msgDiv = document.createElement("div");
            msgDiv.textContent = n.message || n.text || n.title || "Obavestenje";
            const span = document.createElement("span");
            span.className = "notif-time";
            span.textContent = formatDateTime(n.createdAt || n.timestamp);
            li.appendChild(msgDiv);
            li.appendChild(span);
            ul.appendChild(li);
        });
    } catch (err) {
        if (silent) return;
        if (handleAuthError(err)) return;
        showModalMsg("notifMsg", err.status === 502
            ? "Servis za notifikacije trenutno nije dostupan."
            : (err.message || "Ucitavanje notifikacija nije uspelo."));
    }
}

breadcrumbsEl.addEventListener("click", (e) => {
    const idx = e.target.dataset.index;
    if (idx !== undefined) {
        const num = Number(idx);
        if (!Number.isNaN(num) && num < pathStack.length - 1) {
            pathStack = pathStack.slice(0, num + 1);
            searchInput.value = "";
            loadFolderContents();
        }
    }
});

foldersList.addEventListener("click", (e) => {
    const item = e.target.closest(".folder-item");
    if (!item) return;
    const fId = item.dataset.id;
    const folderObj = currentFolders.find(f => f.id === fId);
    if (!folderObj) return;

    if (e.target.classList.contains("rename-folder-btn")) {
        e.stopPropagation();
        renameTarget = { type: "folder", item: folderObj };
        renameInput.value = folderObj.name;
        clearModalMsg("renameMsg");
        renameModal.classList.add("open");
        return;
    }

    pathStack.push({ id: folderObj.id, name: folderObj.name });
    searchInput.value = "";
    loadFolderContents();
});

filesBody.addEventListener("click", (e) => {
    const row = e.target.closest("tr");
    if (!row || !row.dataset.id) return;
    const fileObj = currentFiles.find(f => f.id === row.dataset.id);
    if (!fileObj) return;

    if (e.target.classList.contains("rename-file-btn")) {
        renameTarget = { type: "file", item: fileObj };
        renameInput.value = fileObj.name;
        clearModalMsg("renameMsg");
        renameModal.classList.add("open");
    } else if (e.target.classList.contains("version-file-btn")) {
        versionTargetFile = fileObj;
        document.getElementById("uploadModalTitle").textContent = "Nova verzija: " + fileObj.name;
        document.getElementById("fileInput").value = "";
        clearModalMsg("uploadMsg");
        uploadModal.classList.add("open");
    }
});

document.querySelectorAll(".menu li").forEach(li => {
    li.addEventListener("click", () => {
        document.querySelectorAll(".menu li").forEach(el => el.classList.remove("active"));
        li.classList.add("active");

        const sec = li.dataset.section;
        document.getElementById("view-files").style.display = sec === "files" ? "block" : "none";
        document.getElementById("view-shared").style.display = sec === "shared" ? "block" : "none";
        document.getElementById("view-activity").style.display = sec === "activity" ? "block" : "none";
    });
});

document.getElementById("logoutBtn").addEventListener("click", () => {
    localStorage.removeItem("token");
    localStorage.removeItem("user");
    window.location.href = "login.html";
});

document.getElementById("openNewFolderBtn").addEventListener("click", () => {
    document.getElementById("folderNameInput").value = "";
    clearModalMsg("folderMsg");
    folderModal.classList.add("open");
});

document.getElementById("openUploadBtn").addEventListener("click", () => {
    versionTargetFile = null;
    document.getElementById("uploadModalTitle").textContent = "Otpremi datoteku";
    document.getElementById("fileInput").value = "";
    clearModalMsg("uploadMsg");
    uploadModal.classList.add("open");
});

document.getElementById("openQuotaBtn").addEventListener("click", () => {
    clearModalMsg("quotaMsg");
    loadQuota();
    quotaModal.classList.add("open");
});

document.getElementById("openChangePwBtn").addEventListener("click", () => {
    document.getElementById("oldPwInput").value = "";
    document.getElementById("newPwInput").value = "";
    document.getElementById("repeatNewPwInput").value = "";
    clearModalMsg("changePwMsg");
    changePwModal.classList.add("open");
});

document.getElementById("openNotifBtn").addEventListener("click", () => {
    notifModal.classList.add("open");
    loadNotifications(false);
});

function closeAllModals() {
    document.querySelectorAll(".modal-bg").forEach(m => m.classList.remove("open"));
}

document.querySelectorAll(".close-modal").forEach(btn => {
    btn.addEventListener("click", closeAllModals);
});

document.querySelectorAll(".modal-bg").forEach(bg => {
    bg.addEventListener("click", (e) => {
        if (e.target === bg) closeAllModals();
    });
});

document.getElementById("saveFolderBtn").addEventListener("click", async () => {
    clearModalMsg("folderMsg");
    const name = document.getElementById("folderNameInput").value.trim();
    if (!name) {
        showModalMsg("folderMsg", "Unesite naziv foldera.");
        return;
    }

    const btn = document.getElementById("saveFolderBtn");
    btn.disabled = true;

    try {
        const body = { name, parentId: currentFolderId() };
        await sendRequest("/folders", "POST", body);
        logActivity("Kreiranje foldera", name);
        closeAllModals();
        await loadFolderContents();
    } catch (err) {
        if (handleAuthError(err)) return;
        showModalMsg("folderMsg", err.message || "Kreiranje foldera nije uspelo.");
    } finally {
        btn.disabled = false;
    }
});

document.getElementById("saveUploadBtn").addEventListener("click", async () => {
    clearModalMsg("uploadMsg");
    const input = document.getElementById("fileInput");
    if (!input.files || !input.files.length) {
        showModalMsg("uploadMsg", "Izaberite datoteku.");
        return;
    }

    const file = input.files[0];
    if (file.size > MAX_UPLOAD_BYTES) {
        showModalMsg("uploadMsg", "Datoteka prelazi maksimalnu velicinu od 50 MB.");
        return;
    }

    const btn = document.getElementById("saveUploadBtn");
    btn.disabled = true;

    try {
        const fd = new FormData();
        fd.append("file", file);

        if (versionTargetFile) {
            await sendMultipart("/files/" + encodeURIComponent(versionTargetFile.id) + "/versions", fd, "POST");
            logActivity("Otpremanje nove verzije", versionTargetFile.name);
        } else {
            const pid = currentFolderId();
            if (pid) {
                fd.append("parentId", pid);
            }
            await sendMultipart("/files", fd, "POST");
            logActivity("Otpremanje datoteke", file.name);
        }

        versionTargetFile = null;
        closeAllModals();
        await loadFolderContents();
    } catch (err) {
        if (handleAuthError(err)) return;
        showModalMsg("uploadMsg", err.message || "Otpremanje datoteke nije uspelo.");
    } finally {
        btn.disabled = false;
    }
});

document.getElementById("saveRenameBtn").addEventListener("click", async () => {
    clearModalMsg("renameMsg");
    const newName = renameInput.value.trim();
    if (!newName || !renameTarget) {
        showModalMsg("renameMsg", "Unesite novi naziv.");
        return;
    }

    const btn = document.getElementById("saveRenameBtn");
    btn.disabled = true;

    try {
        const base = renameTarget.type === "folder" ? "/folders/" : "/files/";
        await sendRequest(base + encodeURIComponent(renameTarget.item.id), "PATCH", { name: newName });
        logActivity("Preimenovanje", renameTarget.item.name + " -> " + newName);
        renameTarget = null;
        closeAllModals();
        await loadFolderContents();
    } catch (err) {
        if (handleAuthError(err)) return;
        showModalMsg("renameMsg", err.status === 404 || err.status === 405
            ? "Endpoint za preimenovanje jos nije dostupan na Files servisu."
            : (err.message || "Preimenovanje nije uspelo."));
    } finally {
        btn.disabled = false;
    }
});

document.getElementById("sendQuotaBtn").addEventListener("click", async () => {
    clearModalMsg("quotaMsg");
    const gb = Number(document.getElementById("quotaInput").value);
    if (!Number.isFinite(gb) || gb <= 0 || gb > 100) {
        showModalMsg("quotaMsg", "Unesite ispravnu vrednost kvote u GB (1 - 100).");
        return;
    }

    const requestedBytes = Math.round(gb * 1073741824);
    const btn = document.getElementById("sendQuotaBtn");
    btn.disabled = true;

    try {
        await sendRequest("/quota/requests", "POST", { requestedBytes });
        showModalMsg("quotaMsg", "Zahtev za povecanje kvote je uspesno poslat administratoru.", "success");
        logActivity("Zahtev za povecanje kvote", gb + " GB");
        await loadQuota();
    } catch (err) {
        if (handleAuthError(err)) return;
        if (err.code === "request_pending") {
            showModalMsg("quotaMsg", "Vec imate aktivan zahtev na cekanju.");
        } else if (err.fields && err.fields.requestedBytes) {
            showModalMsg("quotaMsg", err.fields.requestedBytes);
        } else {
            showModalMsg("quotaMsg", err.message || "Slanje zahteva nije uspelo.");
        }
    } finally {
        btn.disabled = false;
    }
});

document.getElementById("saveChangePwBtn").addEventListener("click", async () => {
    clearModalMsg("changePwMsg");
    const currentPassword = document.getElementById("oldPwInput").value;
    const newPassword = document.getElementById("newPwInput").value;
    const newPasswordRepeat = document.getElementById("repeatNewPwInput").value;

    if (!currentPassword) {
        showModalMsg("changePwMsg", "Unesite trenutnu lozinku.");
        return;
    }
    const pwErr = validateStrongPassword(newPassword);
    if (pwErr) {
        showModalMsg("changePwMsg", pwErr);
        return;
    }
    if (newPassword !== newPasswordRepeat) {
        showModalMsg("changePwMsg", "Lozinke se ne poklapaju.");
        return;
    }

    const btn = document.getElementById("saveChangePwBtn");
    btn.disabled = true;

    try {
        await sendRequest("/password/change", "POST", {
            currentPassword,
            newPassword,
            newPasswordRepeat
        });
        showModalMsg("changePwMsg", "Lozinka je uspesno promenjena.", "success");
        logActivity("Promena lozinke", "Korisnicki nalog");
    } catch (err) {
        if (handleAuthError(err)) return;
        if (err.code === "password_too_new") {
            showModalMsg("changePwMsg", "Lozinka mora biti stara bar 1 dan pre nove promene.");
        } else if (err.code === "wrong_password") {
            showModalMsg("changePwMsg", "Trenutna lozinka nije ispravna.");
        } else if (err.fields && err.fields.newPassword) {
            showModalMsg("changePwMsg", err.fields.newPassword);
        } else {
            showModalMsg("changePwMsg", err.message || "Promena lozinke nije uspela.");
        }
    } finally {
        btn.disabled = false;
    }
});

searchInput.addEventListener("input", () => {
    renderDrive(searchInput.value);
});

renderActivities();
loadQuota();
loadFolderContents();
loadNotifications(true);
