# NYCU-G39-CS-Project

區塊鏈產權拍賣轉移系統，使用 Hyperledger Fabric 與 Go chaincode 實作。

## Chaincode

目前先實作「A. 使用者註冊與身分初始化」相關合約。

### 身分鏈：Identity Chaincode

位置：`chaincode/identity`

主要職責：

- `RegisterIdentity() -> string`
  - 由審核機關節點呼叫。
  - 生成使用者 `identityDID`。
  - 建立並綁定該 DID 對應的 PIMgr ledger record。
  - 回傳 `identityDID`。
- `GetPIMgr(userDID) -> string`
  - 使用 `identityDID` 查詢 PIMgr 位址。
  - 在 Fabric 實作中，`PIMgrAddr` 是 ledger key，不是動態部署的 chaincode address。
- `SetProfile(pimgrAddr, userName, idCardNumber, email, phone) -> bool`
  - 寫入使用者個人資料。
- `SetPublicKey(pimgrAddr, publicKey) -> bool`
  - 寫入使用者公鑰。
- `GetPublicKey(userDID) -> string`
  - 查詢 PIMgr 中保存的公鑰。

### 交易鏈：Transaction Identity Registry Chaincode

位置：`chaincode/transaction`

主要職責：

- `RegisterTradingIdentity(identityDID) -> { buyerDID, sellerDID }`
  - 由交易服務節點呼叫。
  - 建立 `identityDID` 對應的 `buyerDID` 與 `sellerDID`。
- `SetPublicKey(identityDID, publicKey) -> bool`
  - 將使用者公鑰同步登錄至交易鏈。
- `GetTradingIdentity(identityDID)`
  - 查詢交易身分映射。
- `GetByBuyerDID(buyerDID)`
  - 由買家 DID 反查交易身分。
- `GetBySellerDID(sellerDID)`
  - 由賣家 DID 反查交易身分。
- `GetPublicKey(identityDID) -> string`
  - 查詢交易鏈上登錄的公鑰。

## A. 使用者註冊與身分初始化流程

1. 使用者端輸入真實姓名、身分證字號、Email、手機號碼。
2. 審核機關節點完成鏈下身分驗證。
3. 使用者端本地生成密鑰對，私鑰留在本地，公鑰交給審核機關節點。
4. 審核機關節點呼叫身分鏈 `RegisterIdentity()`，取得 `identityDID`。
5. 審核機關節點呼叫身分鏈 `GetPIMgr(identityDID)`，取得 `PIMgrAddr`。
6. 審核機關節點使用 `PIMgrAddr` 呼叫身分鏈 `SetProfile(...)` 與 `SetPublicKey(...)` 寫入 PIMgr record。
7. 審核機關節點將 `identityDID` 與 `publicKey` 傳給交易服務節點。
8. 交易服務節點呼叫交易鏈 `RegisterTradingIdentity(identityDID)`，取得 `buyerDID` 與 `sellerDID`。
9. 交易服務節點呼叫交易鏈 `SetPublicKey(identityDID, publicKey)`。
10. 審核機關節點將 `identityDID`、`buyerDID`、`sellerDID` 回傳給使用者端。

## 權限屬性

目前 chaincode 使用 Fabric client identity attribute `role` 做基本權限檢查：

- 身分鏈註冊與 PIMgr 寫入：`role=authority`
- 交易鏈身分註冊與交易鏈公鑰寫入：`role=transactionService`
- 部分查詢允許：`role=authority`、`role=transactionService` 或 `role=verifier`
