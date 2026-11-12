const folderModal = document.getElementById("folderModal");
const uploadModal = document.getElementById("uploadModal");
const renameModal = document.getElementById("renameModal");
const quotaModal = document.getElementById("quotaModal");
const previewModal = document.getElementById("previewModal");
const changePwModal = document.getElementById("changePwModal");
const notifModal = document.getElementById("notifModal");

const foldersList = document.getElementById("foldersList");
const filesBody = document.getElementById("filesBody");
const activityBody = document.getElementById("activityBody");
const breadcrumbsEl = document.getElementById("breadcrumbs");
const searchInput = document.getElementById("searchInput");
const renameInput = document.getElementById("renameInput");

let currentFolderId = "root";
let renameTarget = null;
let versionTargetFileId = null;
let currentPreviewFile = null;

const maxQuotaBytes = 1073741824;
let usedQuotaBytes = 2516582;

const folders = [
    { id: "f1", name: "Dokumentacija", parentId: "root" },
    { id: "f2", name: "Projekti", parentId: "root" },
    { id: "f3", name: "Arhiva", parentId: "f1" }
];

const files = [
    {
        id: "file1",
        name: "Specifikacija.txt",
        folderId: "root",
        owner: "Ja",
        updatedAt: "10.11.2026. 12:30",
        sizeBytes: 2457600,
        version: 1,
        content: "Projektni zadatak: Platforma za skladistenje i deljenje fajlova."
    },
    {
        id: "file2",
        name: "beleske.txt",
        folderId: "root",
        owner: "Ja",
        updatedAt: "09.11.2026. 18:15",
        sizeBytes: 14336,
        version: 1,
        content: "Beleske sa sastanka tima za mikroservise i baze podataka."
    },
    {
        id: "file3",
        name: "Uputstvo.txt",
        folderId: "f1",
        owner: "Ja",
        updatedAt: "10.11.2026. 10:00",
        sizeBytes: 4096,
        version: 1,
        content: "Uputstvo za pokretanje kontejnera pomocu docker compose alata."
    }
];

const activities = [
    { action: "Otpremanje datoteke", item: "Specifikacija.txt", time: "10.11.2026. 12:30" },
    { action: "Kreiranje foldera", item: "Dokumentacija", time: "10.11.2026. 09:15" }
];

const savedUser = JSON.parse(localStorage.getItem("user") || "null");
if (savedUser && savedUser.firstName) {
    document.getElementById("userName").textContent = savedUser.firstName + " " + (savedUser.lastName || "");
    document.getElementById("userAvatar").textContent = savedUser.firstName[0].toUpperCase();
}

function nowFormatted() {
    const d = new Date();
    return d.toLocaleDateString("sr-RS") + " " + d.toLocaleTimeString("sr-RS", { hour: "2-digit", minute: "2-digit" });
}

function formatSize(bytes) {
    if (bytes >= 1048576) {
        return (bytes / 1048576).toFixed(1) + " MB";
    }
    return Math.max(1, Math.round(bytes / 1024)) + " KB";
}

function logActivity(action, item) {
    activities.unshift({ action, item, time: nowFormatted() });
    renderActivities();
}

function updateQuotaUI() {
    usedQuotaBytes = files.reduce((sum, f) => sum + f.sizeBytes, 0);
    const pct = Math.min(100, Math.round((usedQuotaBytes / maxQuotaBytes) * 100));
    document.getElementById("quotaLabel").textContent = formatSize(usedQuotaBytes) + " / 1 GB";
    document.getElementById("quotaFill").style.width = Math.max(2, pct) + "%";
}

function buildPath(folderId) {
    const chain = [];
    let curr = folders.find(f => f.id === folderId);
    while (curr) {
        chain.unshift(curr);
        curr = folders.find(f => f.id === curr.parentId);
    }
    return chain;
}

function renderBreadcrumbs() {
    breadcrumbsEl.innerHTML = "";
    const rootSpan = document.createElement("span");
    rootSpan.textContent = "Moji fajlovi";
    rootSpan.dataset.folder = "root";
    if (currentFolderId === "root") {
        rootSpan.className = "current";
    }
    breadcrumbsEl.appendChild(rootSpan);

    const chain = buildPath(currentFolderId);
    chain.forEach((f, idx) => {
        const sep = document.createTextNode(" / ");
        breadcrumbsEl.appendChild(sep);

        const s = document.createElement("span");
        s.textContent = f.name;
        s.dataset.folder = f.id;
        if (idx === chain.length - 1) {
            s.className = "current";
        }
        breadcrumbsEl.appendChild(s);
    });
}

function renderDrive(filterQuery = "") {
    const q = filterQuery.toLowerCase().trim();
    renderBreadcrumbs();

    foldersList.innerHTML = "";
    const visibleFolders = folders.filter(f => {
        if (q) return f.name.toLowerCase().includes(q);
        return f.parentId === currentFolderId;
    });

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

    filesBody.innerHTML = "";
    const visibleFiles = files.filter(f => {
        if (q) return f.name.toLowerCase().includes(q);
        return f.folderId === currentFolderId;
    });

    visibleFiles.forEach(f => {
        const tr = document.createElement("tr");
        tr.dataset.id = f.id;

        const tdName = document.createElement("td");
        tdName.className = "file-title";
        tdName.textContent = f.name + (f.version > 1 ? " (v" + f.version + ")" : "");

        const tdOwner = document.createElement("td");
        tdOwner.textContent = f.owner;

        const tdDate = document.createElement("td");
        tdDate.textContent = f.updatedAt;

        const tdSize = document.createElement("td");
        tdSize.textContent = formatSize(f.sizeBytes);

        const tdActions = document.createElement("td");
        tdActions.innerHTML = `
            <div class="row-actions">
                <button class="btn btn-outline btn-sm preview-btn">Pregled</button>
                <button class="btn btn-outline btn-sm version-file-btn">Nova verzija</button>
                <button class="btn btn-outline btn-sm rename-file-btn">Preimenuj</button>
                <button class="btn btn-danger btn-sm delete-file-btn">Obrisi</button>
            </div>
        `;

        tr.appendChild(tdName);
        tr.appendChild(tdOwner);
        tr.appendChild(tdDate);
        tr.appendChild(tdSize);
        tr.appendChild(tdActions);
        filesBody.appendChild(tr);
    });

    updateQuotaUI();
}

function renderActivities() {
    if (!activityBody) return;
    activityBody.innerHTML = "";
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

breadcrumbsEl.addEventListener("click", (e) => {
    const folderId = e.target.dataset.folder;
    if (folderId) {
        currentFolderId = folderId;
        searchInput.value = "";
        renderDrive();
    }
});

foldersList.addEventListener("click", (e) => {
    const item = e.target.closest(".folder-item");
    if (!item) return;
    const fId = item.dataset.id;
    const folderObj = folders.find(f => f.id === fId);
    if (!folderObj) return;

    if (e.target.classList.contains("rename-folder-btn")) {
        e.stopPropagation();
        renameTarget = { type: "folder", item: folderObj };
        renameInput.value = folderObj.name;
        renameModal.classList.add("open");
        return;
    }

    currentFolderId = folderObj.id;
    searchInput.value = "";
    renderDrive();
});

filesBody.addEventListener("click", (e) => {
    const row = e.target.closest("tr");
    if (!row) return;
    const fileObj = files.find(f => f.id === row.dataset.id);
    if (!fileObj) return;

    if (e.target.classList.contains("rename-file-btn")) {
        renameTarget = { type: "file", item: fileObj };
        renameInput.value = fileObj.name;
        renameModal.classList.add("open");
    } else if (e.target.classList.contains("version-file-btn")) {
        versionTargetFileId = fileObj.id;
        document.getElementById("fileInput").value = "";
        uploadModal.classList.add("open");
    } else if (e.target.classList.contains("preview-btn")) {
        openPreview(fileObj);
    } else if (e.target.classList.contains("delete-file-btn")) {
        const idx = files.findIndex(f => f.id === fileObj.id);
        if (idx !== -1) {
            files.splice(idx, 1);
            logActivity("Brisanje datoteke", fileObj.name);
            renderDrive(searchInput.value);
        }
    }
});

function openPreview(fileObj) {
    currentPreviewFile = fileObj;
    document.getElementById("previewTitle").textContent = fileObj.name + " (v" + fileObj.version + ")";
    const box = document.getElementById("previewContent");
    box.innerHTML = "";

    if (fileObj.dataUrl && fileObj.name.match(/\.(png|jpg|jpeg|gif|webp)$/i)) {
        const img = document.createElement("img");
        img.src = fileObj.dataUrl;
        img.style.maxWidth = "100%";
        box.appendChild(img);
    } else {
        box.textContent = fileObj.content || "Sadrzaj datoteke: " + fileObj.name;
    }
    previewModal.classList.add("open");
}

function triggerDownload(name, content, dataUrl) {
    const a = document.createElement("a");
    if (dataUrl) {
        a.href = dataUrl;
    } else {
        const blob = new Blob([content || ""], { type: "text/plain;charset=utf-8" });
        a.href = URL.createObjectURL(blob);
    }
    a.download = name;
    document.body.appendChild(a);
    a.click();
    a.remove();
    logActivity("Preuzimanje datoteke", name);
}

document.getElementById("downloadFromPreviewBtn").addEventListener("click", () => {
    if (!currentPreviewFile) return;
    triggerDownload(currentPreviewFile.name, currentPreviewFile.content, currentPreviewFile.dataUrl);
});

document.getElementById("sharedBody").addEventListener("click", (e) => {
    const name = e.target.dataset.name;
    const content = e.target.dataset.content;
    if (!name) return;

    if (e.target.classList.contains("shared-preview-btn")) {
        openPreview({ name, version: 1, content });
    } else if (e.target.classList.contains("shared-download-btn")) {
        triggerDownload(name, content, null);
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
    folderModal.classList.add("open");
});

document.getElementById("openUploadBtn").addEventListener("click", () => {
    versionTargetFileId = null;
    document.getElementById("fileInput").value = "";
    uploadModal.classList.add("open");
});

document.getElementById("openQuotaBtn").addEventListener("click", () => {
    quotaModal.classList.add("open");
});

document.getElementById("openChangePwBtn").addEventListener("click", () => {
    document.getElementById("oldPwInput").value = "";
    document.getElementById("newPwInput").value = "";
    document.getElementById("repeatNewPwInput").value = "";
    document.getElementById("changePwMsg").className = "msg-box";
    changePwModal.classList.add("open");
});

document.getElementById("openNotifBtn").addEventListener("click", async () => {
    notifModal.classList.add("open");
    const badge = document.getElementById("notifBadge");
    if (badge) badge.style.display = "none";

    try {
        const list = await sendRequest("/notifications", "GET");
        if (Array.isArray(list) && list.length > 0) {
            const ul = document.getElementById("notifList");
            ul.innerHTML = "";
            list.forEach(n => {
                const li = document.createElement("li");
                li.textContent = n.message || n.text;
                const span = document.createElement("span");
                span.className = "notif-time";
                span.textContent = n.createdAt || "";
                li.appendChild(span);
                ul.appendChild(li);
            });
        }
    } catch (_) {}
});

document.querySelectorAll(".close-modal").forEach(btn => {
    btn.addEventListener("click", closeAllModals);
});

document.querySelectorAll(".modal-bg").forEach(bg => {
    bg.addEventListener("click", (e) => {
        if (e.target === bg) closeAllModals();
    });
});

function closeAllModals() {
    document.querySelectorAll(".modal-bg").forEach(m => m.classList.remove("open"));
}

document.getElementById("saveFolderBtn").addEventListener("click", () => {
    const name = document.getElementById("folderNameInput").value.trim();
    if (!name) return;

    folders.push({
        id: "f_" + Date.now(),
        name,
        parentId: currentFolderId
    });
    logActivity("Kreiranje foldera", name);
    renderDrive(searchInput.value);
    closeAllModals();
});

document.getElementById("saveUploadBtn").addEventListener("click", () => {
    const input = document.getElementById("fileInput");
    if (!input.files || !input.files.length) return;

    const file = input.files[0];
    if (usedQuotaBytes + file.size > maxQuotaBytes) {
        alert("Premasena je dodeljena kvota skladista.");
        return;
    }

    const reader = new FileReader();
    reader.onload = () => {
        const isImg = /\.(png|jpg|jpeg|gif|webp)$/i.test(file.name);
        let existing = null;
        if (versionTargetFileId) {
            existing = files.find(f => f.id === versionTargetFileId);
        } else {
            existing = files.find(f => f.folderId === currentFolderId && f.name === file.name);
        }

        if (existing) {
            existing.sizeBytes = file.size;
            existing.updatedAt = nowFormatted();
            existing.version += 1;
            if (isImg) {
                existing.dataUrl = reader.result;
            } else {
                existing.content = reader.result;
            }
            logActivity("Otpremanje nove verzije (v" + existing.version + ")", existing.name);
        } else {
            files.unshift({
                id: "file_" + Date.now(),
                name: file.name,
                folderId: currentFolderId,
                owner: "Ja",
                updatedAt: nowFormatted(),
                sizeBytes: file.size,
                version: 1,
                content: isImg ? "" : reader.result,
                dataUrl: isImg ? reader.result : null
            });
            logActivity("Otpremanje datoteke", file.name);
        }

        versionTargetFileId = null;
        renderDrive(searchInput.value);
        closeAllModals();
    };

    if (/\.(png|jpg|jpeg|gif|webp)$/i.test(file.name)) {
        reader.readAsDataURL(file);
    } else {
        reader.readAsText(file);
    }
});

document.getElementById("saveRenameBtn").addEventListener("click", () => {
    const newName = renameInput.value.trim();
    if (!newName || !renameTarget) return;

    const oldName = renameTarget.item.name;
    renameTarget.item.name = newName;
    if (renameTarget.type === "file") {
        renameTarget.item.updatedAt = nowFormatted();
    }
    logActivity("Preimenovanje (" + oldName + " -> " + newName + ")", newName);
    renameTarget = null;
    renderDrive(searchInput.value);
    closeAllModals();
});

document.getElementById("sendQuotaBtn").addEventListener("click", () => {
    const reqGb = document.getElementById("quotaInput").value;
    logActivity("Zahtev za povecanje kvote", reqGb + " GB");
    closeAllModals();
});

document.getElementById("saveChangePwBtn").addEventListener("click", async () => {
    const oldPassword = document.getElementById("oldPwInput").value;
    const newPassword = document.getElementById("newPwInput").value;
    const repeatPassword = document.getElementById("repeatNewPwInput").value;
    const msg = document.getElementById("changePwMsg");

    if (newPassword.length < 10) {
        msg.className = "msg-box error show";
        msg.textContent = "Nova lozinka mora imati najmanje 10 karaktera.";
        return;
    }
    if (newPassword !== repeatPassword) {
        msg.className = "msg-box error show";
        msg.textContent = "Lozinke se ne poklapaju.";
        return;
    }

    try {
        await sendRequest("/password-change", "POST", { oldPassword, newPassword, passwordRepeat: repeatPassword });
        msg.className = "msg-box success show";
        msg.textContent = "Lozinka je uspesno promenjena.";
    } catch (err) {
        msg.className = "msg-box error show";
        msg.textContent = err.message || "Promena lozinke nije uspela (lozinka mora biti bar 1 dan stara).";
    }
});

searchInput.addEventListener("input", () => {
    renderDrive(searchInput.value);
});

renderDrive();
renderActivities();
