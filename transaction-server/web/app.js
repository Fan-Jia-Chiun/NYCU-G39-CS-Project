const state = {
  identityDID: sessionStorage.getItem("demo.identityDID") || "",
  buyerDID: "",
  sellerDID: "",
  sessionToken: sessionStorage.getItem("demo.sessionToken") || "",
  clientURL: localStorage.getItem("demo.clientURL") || "http://127.0.0.1:8090",
  photoHash: "",
};

const el = {
  clientURL: document.querySelector("#clientURL"),
  identityDID: document.querySelector("#identityDID"),
  loadIdentityButton: document.querySelector("#loadIdentityButton"),
  saveIdentityButton: document.querySelector("#saveIdentityButton"),
  saveIdentityFolderButton: document.querySelector("#saveIdentityFolderButton"),
  identityFile: document.querySelector("#identityFile"),
  identityCacheState: document.querySelector("#identityCacheState"),
  identityPath: document.querySelector("#identityPath"),
  cachedBuyerDID: document.querySelector("#cachedBuyerDID"),
  cachedSellerDID: document.querySelector("#cachedSellerDID"),
  tabButtons: document.querySelectorAll(".tab-button"),
  tabPanels: document.querySelectorAll(".tab-panel"),
  userName: document.querySelector("#userName"),
  idCardNumber: document.querySelector("#idCardNumber"),
  email: document.querySelector("#email"),
  phone: document.querySelector("#phone"),
  registerButton: document.querySelector("#registerButton"),
  registerState: document.querySelector("#registerState"),
  registeredUserDID: document.querySelector("#registeredUserDID"),
  registeredPIMgrAddr: document.querySelector("#registeredPIMgrAddr"),
  registeredBuyerDID: document.querySelector("#registeredBuyerDID"),
  registeredSellerDID: document.querySelector("#registeredSellerDID"),
  registerResult: document.querySelector("#registerResult"),
  loginButton: document.querySelector("#loginButton"),
  loginState: document.querySelector("#loginState"),
  accountStatus: document.querySelector("#accountStatus"),
  buyerCreditScore: document.querySelector("#buyerCreditScore"),
  sellerCreditScore: document.querySelector("#sellerCreditScore"),
  verifiedBuyerDID: document.querySelector("#verifiedBuyerDID"),
  verifiedSellerDID: document.querySelector("#verifiedSellerDID"),
  sessionExpiration: document.querySelector("#sessionExpiration"),
  sessionToken: document.querySelector("#sessionToken"),
  assetList: document.querySelector("#assetList"),
  assetListCount: document.querySelector("#assetListCount"),
  currentTransactionList: document.querySelector("#currentTransactionList"),
  currentTransactionCount: document.querySelector("#currentTransactionCount"),
  activeTradeList: document.querySelector("#activeTradeList"),
  activeTradeCount: document.querySelector("#activeTradeCount"),
  historicalTradeList: document.querySelector("#historicalTradeList"),
  historicalTradeCount: document.querySelector("#historicalTradeCount"),
  loginResult: document.querySelector("#loginResult"),
  assetName: document.querySelector("#assetName"),
  assetLocation: document.querySelector("#assetLocation"),
  description: document.querySelector("#description"),
  photo: document.querySelector("#photo"),
  photoName: document.querySelector("#photoName"),
  photoHash: document.querySelector("#photoHash"),
  registerAssetButton: document.querySelector("#registerAssetButton"),
  assetState: document.querySelector("#assetState"),
  assetMessage: document.querySelector("#assetMessage"),
  assetID: document.querySelector("#assetID"),
  photoCID: document.querySelector("#photoCID"),
  assetInfoAddr: document.querySelector("#assetInfoAddr"),
  assetResult: document.querySelector("#assetResult"),
};

init();

function init() {
  el.clientURL.value = state.clientURL;
  el.identityDID.value = state.identityDID;
  el.sessionToken.value = state.sessionToken;
  renderCachedDIDs("cache not loaded");
  renderLoginInitialization({});

  el.tabButtons.forEach((button) => {
    button.addEventListener("click", () => activateTab(button.dataset.tab));
  });
  el.registerButton.addEventListener("click", registerUser);
  el.loginButton.addEventListener("click", login);
  el.registerAssetButton.addEventListener("click", registerAsset);
  el.photo.addEventListener("change", updatePhotoHash);
  el.loadIdentityButton.addEventListener("click", readIdentity);
  el.saveIdentityButton.addEventListener("click", saveIdentity);
  el.saveIdentityFolderButton.addEventListener("click", saveIdentityToFolder);
  el.identityFile.addEventListener("change", markSelectedIdentityFile);
  el.clientURL.addEventListener("change", () => {
    state.clientURL = normalizeClientURL(el.clientURL.value);
    el.clientURL.value = state.clientURL;
    localStorage.setItem("demo.clientURL", state.clientURL);
  });
  el.identityDID.addEventListener("change", () => {
    state.identityDID = el.identityDID.value.trim();
    sessionStorage.setItem("demo.identityDID", state.identityDID);
  });
}

async function readIdentity() {
  if (el.identityFile.files[0]) {
    await loadSelectedIdentityFile();
    return;
  }

  await loadIdentityCache();
}

async function loadIdentityCache() {
  setStatus(el.identityCacheState, "busy", "Reading identity.json");
  try {
    const response = await fetch(apiURL("/api/identity"));
    const body = await readResponse(response);
    if (!body.cacheFound) {
      el.identityPath.textContent = "No saved identity file";
      renderCachedDIDs("no local cache");
      setStatus(el.identityCacheState, "bad", "No identity.json");
      return;
    }

    applyIdentityCache(body, "local cache, not login-verified");
    setStatus(el.identityCacheState, "ok", "identity.json loaded");
  } catch (error) {
    renderCachedDIDs("cache unavailable");
    setStatus(el.identityCacheState, "bad", "Read failed");
  }
}

function markSelectedIdentityFile() {
  const file = el.identityFile.files[0];
  if (!file) {
    setStatus(el.identityCacheState, "idle", "Cache not loaded");
    return;
  }

  renderValueWithNote(el.identityPath, `Selected: ${file.name}`, "not loaded yet");
  setStatus(el.identityCacheState, "busy", "File selected; press Read");
}

async function loadSelectedIdentityFile() {
  const file = el.identityFile.files[0];
  if (!file) {
    return;
  }

  setStatus(el.identityCacheState, "busy", "Reading selected file");
  try {
    const text = await file.text();
    const body = JSON.parse(text);
    applyIdentityCache(body, "selected file, not login-verified");
    renderValueWithNote(el.identityPath, `Selected: ${file.name}`, "full path hidden by browser");
    setStatus(el.identityCacheState, "ok", `Loaded ${file.name}`);
  } catch (error) {
    setStatus(el.identityCacheState, "bad", "Invalid identity file");
  }
}

function applyIdentityCache(body, note) {
  const identityDID = (body.identityDID || body.userDID || "").trim();
  if (!identityDID) {
    throw new Error("identityDID is required");
  }

  state.identityDID = identityDID;
  state.buyerDID = (body.buyerDID || "").trim();
  state.sellerDID = (body.sellerDID || "").trim();
  sessionStorage.setItem("demo.identityDID", state.identityDID);
  el.identityDID.value = state.identityDID;
  if (body.identityPath) {
    el.identityPath.textContent = body.identityPath;
  }
  renderCachedDIDs(note);
}

async function registerUser() {
  const payload = {
    userName: el.userName.value.trim(),
    idCardNumber: el.idCardNumber.value.trim(),
    email: el.email.value.trim(),
    phone: el.phone.value.trim(),
  };

  if (!payload.userName || !payload.idCardNumber || !payload.email || !payload.phone) {
    setStatus(el.registerState, "bad", "Missing fields");
    renderJSON(el.registerResult, {
      success: false,
      message: "userName, idCardNumber, email, and phone are required",
    });
    return;
  }

  setStatus(el.registerState, "busy", "Registering");
  el.registerButton.disabled = true;
  try {
    const response = await postJSON(apiURL("/api/register"), payload);
    state.identityDID = response.identityDID || response.userDID || "";
    state.buyerDID = response.buyerDID || "";
    state.sellerDID = response.sellerDID || "";
    sessionStorage.setItem("demo.identityDID", state.identityDID);

    el.identityDID.value = state.identityDID;
    el.registeredUserDID.textContent = state.identityDID || "-";
    el.registeredPIMgrAddr.textContent = response.pimgrAddr || "-";
    el.registeredBuyerDID.textContent = response.buyerDID || "-";
    el.registeredSellerDID.textContent = response.sellerDID || "-";
    el.identityPath.textContent = response.identityPath || el.identityPath.textContent || "-";
    renderCachedDIDs("ready to save, not login-verified");
    renderJSON(el.registerResult, response);
    setStatus(el.registerState, "ok", "Registered");
    activateTab("loginPanel");
  } catch (error) {
    renderJSON(el.registerResult, errorPayload(error));
    setStatus(el.registerState, "bad", "Failed");
  } finally {
    el.registerButton.disabled = false;
  }
}

async function login() {
  const identityDID = el.identityDID.value.trim();
  if (!identityDID) {
    setStatus(el.loginState, "bad", "Missing DID");
    renderJSON(el.loginResult, { success: false, message: "Identity DID is required" });
    return;
  }

  setStatus(el.loginState, "busy", "Logging in");
  el.loginButton.disabled = true;
  try {
    const response = await postJSON(apiURL("/api/login"), { identityDID });

    state.identityDID = response.identityDID || response.userDID || identityDID;
    state.buyerDID = response.buyerDID || "";
    state.sellerDID = response.sellerDID || "";
    state.sessionToken = response.sessionToken || "";
    sessionStorage.setItem("demo.identityDID", state.identityDID);
    sessionStorage.setItem("demo.sessionToken", state.sessionToken);

    el.identityDID.value = state.identityDID;
    el.sessionToken.value = state.sessionToken;
    renderAccountStatus(response.accountStatus);
    el.buyerCreditScore.textContent = response.creditScores?.buyerCreditScore ?? "-";
    el.sellerCreditScore.textContent = response.creditScores?.sellerCreditScore ?? "-";
    el.verifiedBuyerDID.textContent = response.buyerDID || "-";
    el.verifiedSellerDID.textContent = response.sellerDID || "-";
    renderValueWithNote(el.sessionExpiration, formatTaipeiDateTime(response.expiresAt || response.sessionExpiresAt), "UTC+8");
    el.identityPath.textContent = response.identityPath || el.identityPath.textContent || "-";
    renderCachedDIDs("synced from verified login, ready to save");
    renderLoginInitialization(response);
    renderJSON(el.loginResult, response);
    setStatus(el.loginState, "ok", "Logged in");
  } catch (error) {
    renderJSON(el.loginResult, errorPayload(error));
    setStatus(el.loginState, "bad", "Login failed");
  } finally {
    el.loginButton.disabled = false;
  }
}

async function saveIdentity() {
  const payload = currentIdentityPayload();
  if (!payload.identityDID) {
    setStatus(el.identityCacheState, "bad", "Missing DID");
    return;
  }

  setStatus(el.identityCacheState, "busy", "Saving identity.json");
  try {
    const response = await postJSON(apiURL("/api/identity"), payload);
    applyIdentityCache(response, "saved to identity.json");
    if (response.identityPath) {
      el.identityPath.textContent = response.identityPath;
    }
    setStatus(el.identityCacheState, "ok", "identity.json saved");
  } catch (error) {
    setStatus(el.identityCacheState, "bad", "Save failed");
  }
}

async function saveIdentityToFolder() {
  const payload = currentIdentityPayload();
  if (!payload.identityDID) {
    setStatus(el.identityCacheState, "bad", "Missing DID");
    return;
  }
  if (!window.showDirectoryPicker) {
    setStatus(el.identityCacheState, "bad", "Folder picker unavailable");
    return;
  }

  setStatus(el.identityCacheState, "busy", "Choosing folder");
  try {
    const directory = await window.showDirectoryPicker({ mode: "readwrite" });
    const file = await directory.getFileHandle("identity.json", { create: true });
    const writable = await file.createWritable();
    await writable.write(JSON.stringify(payload, null, 2) + "\n");
    await writable.close();

    renderValueWithNote(el.identityPath, `Selected folder: ${directory.name}`, "identity.json saved");
    renderCachedDIDs("saved to selected folder");
    setStatus(el.identityCacheState, "ok", "identity.json saved");
  } catch (error) {
    if (error.name === "AbortError") {
      setStatus(el.identityCacheState, "idle", "Save cancelled");
      return;
    }
    setStatus(el.identityCacheState, "bad", "Folder save failed");
  }
}

function currentIdentityPayload() {
  state.identityDID = el.identityDID.value.trim() || state.identityDID;
  sessionStorage.setItem("demo.identityDID", state.identityDID);

  return {
    identityDID: state.identityDID,
    buyerDID: state.buyerDID || "",
    sellerDID: state.sellerDID || "",
  };
}

async function updatePhotoHash() {
  const file = el.photo.files[0];
  state.photoHash = "";
  el.photoName.textContent = "-";
  el.photoHash.textContent = "-";

  if (!file) {
    return;
  }

  const bytes = await file.arrayBuffer();
  state.photoHash = await sha256Hex(bytes);
  el.photoName.textContent = file.name;
  el.photoHash.textContent = state.photoHash;
}

async function registerAsset() {
  const identityDID = el.identityDID.value.trim();
  const sessionToken = el.sessionToken.value.trim();
  const assetName = el.assetName.value.trim();
  const assetLocation = el.assetLocation.value.trim();
  const description = el.description.value.trim();
  const file = el.photo.files[0];

  if (!sessionToken || !identityDID || !assetName || !assetLocation || !file) {
    setStatus(el.assetState, "bad", "Missing fields");
    renderJSON(el.assetResult, {
      success: false,
      message: "sessionToken, identityDID, assetName, assetLocation, and photo are required",
    });
    return;
  }

  setStatus(el.assetState, "busy", "Preparing");
  el.registerAssetButton.disabled = true;
  try {
    if (!state.photoHash) {
      const bytes = await file.arrayBuffer();
      state.photoHash = await sha256Hex(bytes);
      el.photoHash.textContent = state.photoHash;
      el.photoName.textContent = file.name;
    }

    const form = new FormData();
    form.append("sessionToken", sessionToken);
    form.append("identityDID", identityDID);
    form.append("assetName", assetName);
    form.append("assetLocation", assetLocation);
    form.append("description", description);
    form.append("photo", file, file.name);

    setStatus(el.assetState, "busy", "Registering");
    const response = await postForm(apiURL("/api/assets/register"), form);

    state.photoHash = response.photoHash || state.photoHash;
    el.photoHash.textContent = state.photoHash || "-";
    el.assetMessage.textContent = response.message || "-";
    el.assetID.textContent = response.assetID || "-";
    el.photoCID.textContent = response.photoCID || "-";
    el.assetInfoAddr.textContent = response.assetInfoAddr || "-";
    renderJSON(el.assetResult, response);
    setStatus(el.assetState, "ok", "Registered");
  } catch (error) {
    renderJSON(el.assetResult, errorPayload(error));
    el.assetMessage.textContent = "Failure";
    setStatus(el.assetState, "bad", "Failed");
  } finally {
    el.registerAssetButton.disabled = false;
  }
}

async function postJSON(url, payload) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  return readResponse(response);
}

async function postForm(url, form) {
  const response = await fetch(url, {
    method: "POST",
    body: form,
  });
  return readResponse(response);
}

async function readResponse(response) {
  const text = await response.text();
  let body;
  try {
    body = text ? JSON.parse(text) : {};
  } catch {
    body = { message: text };
  }

  if (!response.ok) {
    const error = new Error(body.message || response.statusText);
    error.status = response.status;
    error.body = body;
    throw error;
  }

  return body;
}

async function sha256Hex(arrayBuffer) {
  const digest = await crypto.subtle.digest("SHA-256", arrayBuffer);
  const bytes = Array.from(new Uint8Array(digest));
  return bytes.map((value) => value.toString(16).padStart(2, "0")).join("");
}

function apiURL(path) {
  return `${normalizeClientURL(el.clientURL.value)}${path}`;
}

function normalizeClientURL(value) {
  return (value || "http://127.0.0.1:8090").trim().replace(/\/+$/, "");
}

function renderCachedDIDs(note) {
  renderValueWithNote(el.cachedBuyerDID, state.buyerDID || "-", note);
  renderValueWithNote(el.cachedSellerDID, state.sellerDID || "-", note);
}

function renderValueWithNote(node, value, note) {
  node.textContent = "";

  const valueNode = document.createElement("span");
  valueNode.className = "value-main";
  valueNode.textContent = value || "-";

  const noteNode = document.createElement("span");
  noteNode.className = "value-note";
  noteNode.textContent = note ? `(${note})` : "";

  node.append(valueNode, noteNode);
}

function formatTaipeiDateTime(value) {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("zh-TW", {
    timeZone: "Asia/Taipei",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

function activateTab(panelID) {
  el.tabButtons.forEach((button) => {
    button.classList.toggle("active", button.dataset.tab === panelID);
  });
  el.tabPanels.forEach((panel) => {
    panel.classList.toggle("active", panel.id === panelID);
  });
}

function setStatus(node, kind, text) {
  node.className = `status-tag ${kind}`;
  node.textContent = text;
}

function renderJSON(node, value) {
  node.textContent = JSON.stringify(value, null, 2);
}

function errorPayload(error) {
  return {
    success: false,
    status: error.status || "-",
    message: error.message,
    body: error.body || null,
  };
}

function renderAccountStatus(status) {
  if (status === 0) {
    renderValueWithNote(el.accountStatus, "Available - account can use transaction services", "code 0");
    return;
  }
  if (status === 1) {
    renderValueWithNote(el.accountStatus, "Disabled - account cannot use transaction services", "code 1");
    return;
  }
  if (status === 2) {
    renderValueWithNote(el.accountStatus, "Deregistered - account has been removed", "code 2");
    return;
  }

  renderValueWithNote(el.accountStatus, status ?? "-", "");
}

function renderLoginInitialization(response) {
  renderAssets(response.assets || []);
  renderTradeInfoList(
    el.currentTransactionList,
    el.currentTransactionCount,
    response.currentActiveTransactions || [],
    "No available active transactions",
  );
  renderUserTrades(
    el.activeTradeList,
    el.activeTradeCount,
    response.activeTrades || [],
    "No active trades",
  );
  renderUserTrades(
    el.historicalTradeList,
    el.historicalTradeCount,
    response.historicalTrades || [],
    "No historical trades",
  );
}

function renderAssets(assets) {
  renderTable(
    el.assetList,
    el.assetListCount,
    assets,
    [
      ["Asset Name", (asset) => assetInfoValue(asset, "assetName")],
      ["Asset ID", (asset) => asset.assetID || asset.assetAddr || "-"],
      ["Legal Status", (asset) => legalStatusLabel(asset.legalStatus)],
      ["Photo Link", (asset) => assetPhotoLinkNode(asset)],
      ["Asset Location", (asset) => assetInfoValue(asset, "assetLocation")],
      ["Registration Time", (asset) => assetRegistrationTimeLabel(assetInfoValue(asset, "registrationTime"))],
      ["Photo URL", (asset) => assetInfoValue(asset, "photoUrl")],
      ["Description", (asset) => assetInfoValue(asset, "description")],
      ["Asset Certificate Addr", (asset) => asset.assetAddr || "-"],
      ["AssetInfoAddr / CID", (asset) => asset.assetInfoAddr || "-"],
    ],
    "No assets",
  );
}

function renderTradeInfoList(container, countNode, trades, emptyText) {
  renderTable(
    container,
    countNode,
    trades,
    [
      ["Trade ID", (trade) => displayValue(trade.tradeID)],
      ["Asset ID", (trade) => trade.assetID || "-"],
      ["Status", (trade) => transactionStatusLabel(trade.transactionStatus)],
      ["Mode", (trade) => transactionModeLabel(trade.transactionMode)],
      ["Current Highest Price", (trade) => displayValue(trade.currentHighestPrice)],
    ],
    emptyText,
  );
}

function renderUserTrades(container, countNode, trades, emptyText) {
  renderTable(
    container,
    countNode,
    trades,
    [
      ["Trade ID", (trade) => displayValue(trade.tradeID)],
      ["Role", (trade) => transactionRoleLabel(trade.transactionRole)],
      ["Active", (trade) => activeFlagLabel(trade.isActive)],
      ["Asset ID", (trade) => trade.tradeInfo?.assetID || "-"],
      ["Status", (trade) => transactionStatusLabel(trade.tradeInfo?.transactionStatus)],
      ["Mode", (trade) => transactionModeLabel(trade.tradeInfo?.transactionMode)],
      ["Current Highest Price", (trade) => displayValue(trade.tradeInfo?.currentHighestPrice)],
    ],
    emptyText,
  );
}

function renderTable(container, countNode, rows, columns, emptyText) {
  countNode.textContent = String(rows.length);
  container.textContent = "";
  container.classList.toggle("empty", rows.length === 0);

  if (rows.length === 0) {
    container.textContent = emptyText;
    return;
  }

  const table = document.createElement("table");
  table.className = "record-table";

  const thead = document.createElement("thead");
  const headerRow = document.createElement("tr");
  columns.forEach(([label]) => {
    const th = document.createElement("th");
    th.textContent = label;
    headerRow.append(th);
  });
  thead.append(headerRow);

  const tbody = document.createElement("tbody");
  rows.forEach((row) => {
    const tr = document.createElement("tr");
    columns.forEach(([, readValue]) => {
      const td = document.createElement("td");
      const value = readValue(row);
      if (value instanceof Node) {
        td.append(value);
      } else {
        td.textContent = displayValue(value);
      }
      tr.append(td);
    });
    tbody.append(tr);
  });

  table.append(thead, tbody);
  container.append(table);
}

function assetPhotoLinkNode(asset) {
  const photoURL =
    asset.photoGatewayUrl ||
    asset.photoGatewayURL ||
    ipfsGatewayURL(asset.photoCID || assetInfoValue(asset, "photoUrl") || asset.photoUrl);
  if (!photoURL) {
    return document.createTextNode("-");
  }

  const link = document.createElement("a");
  link.className = "asset-photo-link";
  link.href = photoURL;
  link.target = "_blank";
  link.rel = "noreferrer";
  link.textContent = "click";

  return link;
}

function assetInfoValue(asset, field) {
  return asset.assetInfo?.[field] ?? asset[field] ?? "";
}

function ipfsGatewayURL(value) {
  const cid = normalizeIPFSCID(value);
  if (!cid) {
    return "";
  }

  return `http://127.0.0.1:8080/ipfs/${encodeURIComponent(cid)}`;
}

function normalizeIPFSCID(value) {
  return String(value || "")
    .trim()
    .replace(/^ipfs:\/\//, "")
    .replace(/^\/ipfs\//, "");
}

function assetRegistrationTimeLabel(value) {
  if (!value || typeof value !== "object") {
    return "-";
  }

  const year = Number(value.year || 0);
  const month = Number(value.month || 0);
  const day = Number(value.day || 0);
  const hour = Number(value.hour || 0);
  const minute = Number(value.minute || 0);
  const second = Number(value.second || 0);
  if (!year || !month || !day) {
    return "-";
  }

  return `${year}/${pad2(month)}/${pad2(day)} ${pad2(hour)}:${pad2(minute)}:${pad2(second)} UTC`;
}

function pad2(value) {
  return String(value).padStart(2, "0");
}

function displayValue(value) {
  if (value === null || value === undefined || value === "") {
    return "-";
  }

  return String(value);
}

function legalStatusLabel(status) {
  if (status === 0) {
    return "Normal (code 0)";
  }

  return `Unknown (code ${displayValue(status)})`;
}

function transactionRoleLabel(role) {
  if (role === 0) {
    return "Buyer / code 0";
  }
  if (role === 1) {
    return "Seller / code 1";
  }
  if (role === 2) {
    return "Participant / code 2";
  }

  return `Code ${displayValue(role)}`;
}

function activeFlagLabel(isActive) {
  if (isActive === true) {
    return "Active";
  }
  if (isActive === false) {
    return "Inactive";
  }

  return "-";
}

function transactionStatusLabel(status) {
  const labels = {
    5: "Completed / code 5",
    6: "Cancelled / code 6",
    9: "Returned / code 9",
    10: "Rejected / code 10",
  };
  if (Object.hasOwn(labels, status)) {
    return labels[status];
  }
  if (status === null || status === undefined || status === "") {
    return "-";
  }

  return `Active status / code ${status}`;
}

function transactionModeLabel(mode) {
  if (mode === null || mode === undefined || mode === "") {
    return "-";
  }

  return `Mode code ${mode}`;
}
