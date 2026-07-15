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

  el.tabButtons.forEach((button) => {
    button.addEventListener("click", () => activateTab(button.dataset.tab));
  });
  el.registerButton.addEventListener("click", registerUser);
  el.loginButton.addEventListener("click", login);
  el.registerAssetButton.addEventListener("click", registerAsset);
  el.photo.addEventListener("change", updatePhotoHash);
  el.loadIdentityButton.addEventListener("click", loadIdentityCache);
  el.identityFile.addEventListener("change", loadSelectedIdentityFile);
  el.clientURL.addEventListener("change", () => {
    state.clientURL = normalizeClientURL(el.clientURL.value);
    el.clientURL.value = state.clientURL;
    localStorage.setItem("demo.clientURL", state.clientURL);
  });
  el.identityDID.addEventListener("change", () => {
    state.identityDID = el.identityDID.value.trim();
    sessionStorage.setItem("demo.identityDID", state.identityDID);
  });
  loadIdentityCache();
}

async function loadIdentityCache() {
  setStatus(el.identityCacheState, "busy", "Reading default file");
  try {
    const response = await fetch(apiURL("/api/identity"));
    const body = await readResponse(response);
    if (!body.cacheFound) {
      el.identityPath.textContent = "No saved identity file";
      renderCachedDIDs("no local cache");
      setStatus(el.identityCacheState, "bad", "No default file");
      return;
    }

    applyIdentityCache(body, "local cache, not login-verified");
    setStatus(el.identityCacheState, "ok", "Default file loaded");
  } catch (error) {
    renderCachedDIDs("cache unavailable");
    setStatus(el.identityCacheState, "bad", "Read failed");
  }
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
    el.identityPath.textContent = `Selected: ${file.name} (full path hidden by browser)`;
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
    renderCachedDIDs("saved local cache, not login-verified");
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
    el.accountStatus.textContent = formatAccountStatus(response.accountStatus);
    el.buyerCreditScore.textContent = response.creditScores?.buyerCreditScore ?? "-";
    el.sellerCreditScore.textContent = response.creditScores?.sellerCreditScore ?? "-";
    el.verifiedBuyerDID.textContent = response.buyerDID || "-";
    el.verifiedSellerDID.textContent = response.sellerDID || "-";
    el.sessionExpiration.textContent = response.expiresAt || response.sessionExpiresAt || "-";
    el.identityPath.textContent = response.identityPath || el.identityPath.textContent || "-";
    renderCachedDIDs("synced from verified login");
    renderJSON(el.loginResult, response);
    setStatus(el.loginState, "ok", "Logged in");
    activateTab("assetPanel");
  } catch (error) {
    renderJSON(el.loginResult, errorPayload(error));
    setStatus(el.loginState, "bad", "Login failed");
  } finally {
    el.loginButton.disabled = false;
  }
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
  el.cachedBuyerDID.textContent = formatCachedDID(state.buyerDID, note);
  el.cachedSellerDID.textContent = formatCachedDID(state.sellerDID, note);
}

function formatCachedDID(value, note) {
  if (!value) {
    return `- (${note})`;
  }

  return `${value} (${note})`;
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

function formatAccountStatus(status) {
  if (status === 0) return "0 Available";
  if (status === 1) return "1 Disabled";
  if (status === 2) return "2 Deregistered";
  return status ?? "-";
}
