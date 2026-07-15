const state = {
  userDID: localStorage.getItem("demo.userDID") || "",
  sessionToken: localStorage.getItem("demo.sessionToken") || "",
  helperURL: localStorage.getItem("demo.helperURL") || "http://127.0.0.1:8090",
  photoHash: "",
};

const el = {
  identityDID: document.querySelector("#identityDID"),
  helperURL: document.querySelector("#helperURL"),
  loginButton: document.querySelector("#loginButton"),
  loginState: document.querySelector("#loginState"),
  accountStatus: document.querySelector("#accountStatus"),
  buyerCreditScore: document.querySelector("#buyerCreditScore"),
  sellerCreditScore: document.querySelector("#sellerCreditScore"),
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
  el.identityDID.value = state.userDID;
  el.helperURL.value = state.helperURL;
  el.sessionToken.value = state.sessionToken;

  el.loginButton.addEventListener("click", login);
  el.registerAssetButton.addEventListener("click", registerAsset);
  el.photo.addEventListener("change", updatePhotoHash);
  el.helperURL.addEventListener("change", () => {
    state.helperURL = normalizeHelperURL(el.helperURL.value);
    el.helperURL.value = state.helperURL;
    localStorage.setItem("demo.helperURL", state.helperURL);
  });
}

async function login() {
  const identityDID = el.identityDID.value.trim();
  if (!identityDID) {
    setStatus(el.loginState, "bad", "Missing DID");
    renderJSON(el.loginResult, { success: false, message: "Identity DID is required" });
    return;
  }

  setStatus(el.loginState, "busy", "Signing");
  el.loginButton.disabled = true;
  try {
    const helperURL = normalizeHelperURL(el.helperURL.value);
    const signed = await postJSON(`${helperURL}/sign/login`, { identityDID });

    setStatus(el.loginState, "busy", "Logging in");
    const response = await postJSON("/login", {
      userDID: signed.userDID,
      timestamp: signed.timestamp,
      signature: signed.signature,
    });

    state.userDID = response.userDID;
    state.sessionToken = response.sessionToken || "";
    localStorage.setItem("demo.userDID", state.userDID);
    localStorage.setItem("demo.sessionToken", state.sessionToken);

    el.identityDID.value = state.userDID;
    el.sessionToken.value = state.sessionToken;
    el.accountStatus.textContent = formatAccountStatus(response.accountStatus);
    el.buyerCreditScore.textContent = response.creditScores?.buyerCreditScore ?? "-";
    el.sellerCreditScore.textContent = response.creditScores?.sellerCreditScore ?? "-";
    el.sessionExpiration.textContent = response.sessionExpiresAt || "-";
    renderJSON(el.loginResult, response);
    setStatus(el.loginState, "ok", "Logged in");
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

  setStatus(el.assetState, "busy", "Hashing");
  el.registerAssetButton.disabled = true;
  try {
    if (!state.photoHash) {
      const bytes = await file.arrayBuffer();
      state.photoHash = await sha256Hex(bytes);
      el.photoHash.textContent = state.photoHash;
      el.photoName.textContent = file.name;
    }

    const helperURL = normalizeHelperURL(el.helperURL.value);
    setStatus(el.assetState, "busy", "Signing");
    const signed = await postJSON(`${helperURL}/sign/register-asset`, {
      identityDID,
      assetName,
      assetLocation,
      description,
      photoHash: state.photoHash,
    });

    const form = new FormData();
    form.append("sessionToken", sessionToken);
    form.append("identityDID", identityDID);
    form.append("assetName", assetName);
    form.append("assetLocation", assetLocation);
    form.append("description", description);
    form.append("timestamp", signed.timestamp);
    form.append("photoHash", state.photoHash);
    form.append("signature", signed.signature);
    form.append("photo", file, file.name);

    setStatus(el.assetState, "busy", "Registering");
    const response = await postForm("/assets", form);

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

function normalizeHelperURL(value) {
  return (value || "http://127.0.0.1:8090").trim().replace(/\/+$/, "");
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
