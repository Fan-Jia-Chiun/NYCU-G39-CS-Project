const state = {
  userDID: sessionStorage.getItem("demo.userDID") || "",
  buyerDID: "",
  sellerDID: "",
  sessionToken: sessionStorage.getItem("demo.sessionToken") || "",
  clientURL: localStorage.getItem("demo.clientURL") || "http://127.0.0.1:8090",
  uiMode: localStorage.getItem("demo.uiMode") || "developer",
  photoHash: "",
  lastLoginResponse: {},
};

const el = {
  userModeButton: document.querySelector("#userModeButton"),
  developerModeButton: document.querySelector("#developerModeButton"),
  clientURL: document.querySelector("#clientURL"),
  userDID: document.querySelector("#userDID"),
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
  loginUserDID: document.querySelector("#loginUserDID"),
  copyUserDIDButton: document.querySelector("#copyUserDIDButton"),
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
  assetInfoCID: document.querySelector("#assetInfoCID"),
  assetResult: document.querySelector("#assetResult"),
};

init();

function init() {
  el.clientURL.value = state.clientURL;
  el.userDID.value = state.userDID;
  el.sessionToken.value = state.sessionToken;
  renderCachedDIDs("cache not loaded");
  renderLoginInitialization({});
  applyUIMode(state.uiMode);

  el.userModeButton.addEventListener("click", () => setUIMode("user"));
  el.developerModeButton.addEventListener("click", () => setUIMode("developer"));
  el.copyUserDIDButton.addEventListener("click", copyLoginUserDID);
  el.tabButtons.forEach((button) => {
    button.addEventListener("click", () => activateTab(button.dataset.tab));
  });
  el.registerButton.addEventListener("click", registerUser);
  el.loginButton.addEventListener("click", login);
  el.registerAssetButton.addEventListener("click", registerAsset);
  el.photo.addEventListener("change", updatePhotoHash);
  el.saveIdentityFolderButton.addEventListener("click", saveIdentityToFolder);
  el.identityFile.addEventListener("change", loadSelectedIdentityFile);
  el.clientURL.addEventListener("change", () => {
    state.clientURL = normalizeClientURL(el.clientURL.value);
    el.clientURL.value = state.clientURL;
    localStorage.setItem("demo.clientURL", state.clientURL);
  });
  el.userDID.addEventListener("change", () => {
    state.userDID = el.userDID.value.trim();
    sessionStorage.setItem("demo.userDID", state.userDID);
  });
}

async function loadSelectedIdentityFile() {
  const file = el.identityFile.files[0];
  if (!file) {
    setStatus(el.identityCacheState, "idle", "Cache not loaded");
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
  const userDID = (body.userDID || "").trim();
  if (!userDID) {
    throw new Error("userDID is required");
  }

  state.userDID = userDID;
  state.buyerDID = (body.buyerDID || "").trim();
  state.sellerDID = (body.sellerDID || "").trim();
  sessionStorage.setItem("demo.userDID", state.userDID);
  el.userDID.value = state.userDID;
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
    state.userDID = response.userDID || "";
    state.buyerDID = response.buyerDID || "";
    state.sellerDID = response.sellerDID || "";
    sessionStorage.setItem("demo.userDID", state.userDID);

    el.userDID.value = state.userDID;
    el.registeredUserDID.textContent = state.userDID || "-";
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
  const userDID = el.userDID.value.trim();
  if (!userDID) {
    setStatus(el.loginState, "bad", "Missing DID");
    renderJSON(el.loginResult, { success: false, message: "User DID is required" });
    return;
  }

  setStatus(el.loginState, "busy", "Logging in");
  el.loginButton.disabled = true;
  try {
    const response = await postJSON(apiURL("/api/login"), { userDID });

    state.userDID = response.userDID || userDID;
    state.buyerDID = response.buyerDID || "";
    state.sellerDID = response.sellerDID || "";
    state.sessionToken = response.sessionToken || "";
    sessionStorage.setItem("demo.userDID", state.userDID);
    sessionStorage.setItem("demo.sessionToken", state.sessionToken);

    el.userDID.value = state.userDID;
    el.sessionToken.value = state.sessionToken;
    renderAccountStatus(response.accountStatus);
    renderLoginUserDID(state.userDID);
    el.buyerCreditScore.textContent = response.creditScores?.buyerCreditScore ?? "-";
    el.sellerCreditScore.textContent = response.creditScores?.sellerCreditScore ?? "-";
    el.verifiedBuyerDID.textContent = response.buyerDID || "-";
    el.verifiedSellerDID.textContent = response.sellerDID || "-";
    renderValueWithNote(el.sessionExpiration, formatTaipeiDateTime(response.expiresAt || response.sessionExpiresAt), "UTC+8");
    el.identityPath.textContent = response.identityPath || el.identityPath.textContent || "-";
    renderCachedDIDs("synced from verified login, ready to save");
    state.lastLoginResponse = response;
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

function setUIMode(mode) {
  state.uiMode = mode === "user" ? "user" : "developer";
  localStorage.setItem("demo.uiMode", state.uiMode);
  applyUIMode(state.uiMode);
  if (Object.hasOwn(state.lastLoginResponse, "accountStatus")) {
    renderAccountStatus(state.lastLoginResponse.accountStatus);
  }
  renderLoginInitialization(state.lastLoginResponse);
}

function applyUIMode(mode) {
  document.body.dataset.mode = mode === "user" ? "user" : "developer";
  el.userModeButton.classList.toggle("active", mode === "user");
  el.developerModeButton.classList.toggle("active", mode !== "user");
}

function isDeveloperMode() {
  return state.uiMode !== "user";
}

async function copyLoginUserDID() {
  const userDID = state.userDID || el.userDID.value.trim();
  if (!userDID) {
    return;
  }

  try {
    await navigator.clipboard.writeText(userDID);
    el.copyUserDIDButton.textContent = "Copied";
    setTimeout(() => {
      el.copyUserDIDButton.textContent = "Copy";
    }, 1200);
  } catch {
    el.copyUserDIDButton.textContent = "Copy failed";
    setTimeout(() => {
      el.copyUserDIDButton.textContent = "Copy";
    }, 1200);
  }
}

async function saveIdentityToFolder() {
  const payload = currentIdentityPayload();
  if (!payload.userDID) {
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
  state.userDID = el.userDID.value.trim() || state.userDID;
  sessionStorage.setItem("demo.userDID", state.userDID);

  return {
    userDID: state.userDID,
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
  const userDID = el.userDID.value.trim();
  const sessionToken = el.sessionToken.value.trim();
  const assetName = el.assetName.value.trim();
  const assetLocation = el.assetLocation.value.trim();
  const description = el.description.value.trim();
  const file = el.photo.files[0];

  if (!sessionToken || !userDID || !assetName || !assetLocation || !file) {
    setStatus(el.assetState, "bad", "Missing fields");
    renderJSON(el.assetResult, {
      success: false,
      message: "sessionToken, userDID, assetName, assetLocation, and photo are required",
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
    form.append("userDID", userDID);
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
    el.assetInfoCID.textContent = response.assetInfoCID || "-";
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
    renderValueWithNote(
      el.accountStatus,
      "Available - account can use transaction services",
      isDeveloperMode() ? "code 0" : "",
    );
    return;
  }
  if (status === 1) {
    renderValueWithNote(
      el.accountStatus,
      "Disabled - account cannot use transaction services",
      isDeveloperMode() ? "code 1" : "",
    );
    return;
  }
  if (status === 2) {
    renderValueWithNote(
      el.accountStatus,
      "Deregistered - account has been removed",
      isDeveloperMode() ? "code 2" : "",
    );
    return;
  }

  renderValueWithNote(el.accountStatus, status ?? "-", "");
}

function renderLoginUserDID(userDID) {
  if (!userDID) {
    el.loginUserDID.textContent = "-";
    el.loginUserDID.title = "";
    return;
  }

  el.loginUserDID.textContent = isDeveloperMode() ? userDID : shortenDID(userDID);
  el.loginUserDID.title = userDID;
}

function shortenDID(value) {
  if (!value || value.length <= 28) {
    return value || "-";
  }

  return `${value.slice(0, 18)}...${value.slice(-10)}`;
}

function renderLoginInitialization(response) {
  const assets = response.assets || [];
  const assetNameMap = buildAssetNameMap(assets);
  renderLoginUserDID(state.userDID);
  renderAssets(assets);
  renderTradeInfoList(
    el.currentTransactionList,
    el.currentTransactionCount,
    response.currentActiveTransactions || [],
    assetNameMap,
    "No available active transactions",
  );
  renderUserTrades(
    el.activeTradeList,
    el.activeTradeCount,
    response.activeTrades || [],
    assetNameMap,
    "No active trades",
  );
  renderUserTrades(
    el.historicalTradeList,
    el.historicalTradeCount,
    response.historicalTrades || [],
    assetNameMap,
    "No historical trades",
  );
}

function renderAssets(assets) {
  const columns = [
    ["Asset Name", (asset) => assetInfoValue(asset, "assetName")],
    ["Legal Status", (asset) => legalStatusLabel(asset.legalStatus)],
    ["View", (asset) => assetPhotoLinkNode(asset)],
    ["Asset Location", (asset) => assetInfoValue(asset, "assetLocation")],
    ["Registration Time (UTC+8)", (asset) => assetRegistrationTimeLabel(assetInfoValue(asset, "registrationTime"))],
    ["Description", (asset) => assetInfoValue(asset, "description")],
  ];
  if (isDeveloperMode()) {
    columns.push(
      ["Asset ID", (asset) => asset.assetID || asset.assetAddr || "-"],
      ["Asset Certificate Address", (asset) => asset.assetAddr || "-"],
      ["AssetInfoCID", (asset) => asset.assetInfoCID || "-"],
      ["IPFS CID", (asset) => asset.photoCID || normalizeIPFSCID(assetInfoValue(asset, "photoUrl")) || "-"],
      ["Photo URL", (asset) => photoLinkURL(asset) || assetInfoValue(asset, "photoUrl") || "-"],
    );
  }

  renderTable(
    el.assetList,
    el.assetListCount,
    assets,
    columns,
    "No assets",
  );
}

function renderTradeInfoList(container, countNode, trades, assetNameMap, emptyText) {
  const columns = [
    ["Property Name", (trade) => propertyNameForTrade(trade, assetNameMap)],
    ["Trade Mode", (trade) => transactionModeLabel(trade.transactionMode)],
    ["Current Price", (trade) => displayValue(trade.currentHighestPrice)],
    ["End Time", (trade) => trade.endTime || trade.endTimeText || "-"],
    ["Action", () => actionNode("View")],
  ];
  if (isDeveloperMode()) {
    columns.push(
      ["Trade ID", (trade) => displayValue(trade.tradeID)],
      ["Asset ID", (trade) => trade.assetID || "-"],
      ["Transaction Status", (trade) => transactionStatusLabel(trade.transactionStatus)],
      ["Transaction Mode", (trade) => transactionModeLabel(trade.transactionMode)],
    );
  }

  renderTable(
    container,
    countNode,
    trades,
    columns,
    emptyText,
  );
}

function renderUserTrades(container, countNode, trades, assetNameMap, emptyText) {
  const columns = [
    ["Trade ID", (trade) => displayValue(trade.tradeID)],
    ["Property Name", (trade) => propertyNameForTrade(trade.tradeInfo || {}, assetNameMap)],
    ["Transaction Role", (trade) => transactionRoleLabel(trade.transactionRole)],
    ["Current Status", (trade) => transactionStatusLabel(trade.tradeInfo?.transactionStatus)],
    ["View", () => actionNode("View")],
  ];
  if (isDeveloperMode()) {
    columns.push(
      ["Asset ID", (trade) => trade.tradeInfo?.assetID || "-"],
      ["Transaction Status", (trade) => transactionStatusLabel(trade.tradeInfo?.transactionStatus)],
      ["Transaction Mode", (trade) => transactionModeLabel(trade.tradeInfo?.transactionMode)],
      ["Is Active", (trade) => activeFlagLabel(trade.isActive)],
    );
  }

  renderTable(
    container,
    countNode,
    trades,
    columns,
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
  table.classList.toggle("developer-table", isDeveloperMode());

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
  const photoURL = photoLinkURL(asset);
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

function photoLinkURL(asset) {
  return (
    normalizePhotoURL(asset.photoGatewayUrl) ||
    normalizePhotoURL(asset.photoGatewayURL) ||
    normalizePhotoURL(assetInfoValue(asset, "photoUrl")) ||
    normalizePhotoURL(asset.photoUrl) ||
    ipfsGatewayURL(asset.photoCID)
  );
}

function normalizePhotoURL(value) {
  const text = String(value || "").trim();
  if (!text) {
    return "";
  }
  if (/^https?:\/\//i.test(text)) {
    return text;
  }

  return ipfsGatewayURL(text);
}

function actionNode(label) {
  const button = document.createElement("button");
  button.className = "table-action-button";
  button.type = "button";
  button.textContent = label;

  return button;
}

function buildAssetNameMap(assets) {
  const names = new Map();
  assets.forEach((asset) => {
    const name = assetInfoValue(asset, "assetName");
    if (asset.assetID && name) {
      names.set(asset.assetID, name);
    }
  });

  return names;
}

function propertyNameForTrade(trade, assetNameMap) {
  return trade.propertyName || trade.assetName || assetNameMap.get(trade.assetID) || "-";
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

  return formatTaipeiDateTime(new Date(Date.UTC(year, month - 1, day, hour, minute, second)).toISOString());
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
    return isDeveloperMode() ? "Normal (code 0)" : "Normal";
  }

  return isDeveloperMode() ? `Unknown (code ${displayValue(status)})` : "Unknown";
}

function transactionRoleLabel(role) {
  if (role === 0) {
    return isDeveloperMode() ? "Buyer (code 0)" : "Buyer";
  }
  if (role === 1) {
    return isDeveloperMode() ? "Seller (code 1)" : "Seller";
  }
  if (role === 2) {
    return isDeveloperMode() ? "Participant (code 2)" : "Participant";
  }

  return isDeveloperMode() ? `Code ${displayValue(role)}` : "Unknown";
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
    return isDeveloperMode() ? labels[status] : labels[status].replace(/ \/ code \d+$/, "");
  }
  if (status === null || status === undefined || status === "") {
    return "-";
  }

  return isDeveloperMode() ? `Active status (code ${status})` : "Active";
}

function transactionModeLabel(mode) {
  if (mode === null || mode === undefined || mode === "") {
    return "-";
  }

  return isDeveloperMode() ? `Mode code ${mode}` : "Transaction";
}
