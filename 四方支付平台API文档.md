# API 接口文档

- **基础地址**：`https://example.com`
- **调用方式**：HTTP `POST`（`Content-Type: application/x-www-form-urlencoded`）
- **编码**：UTF-8
- **返回格式**：JSON

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

`code = 0` 表示成功；非 0 为失败，错误原因见 `message`。

---

## 1. 鉴权方式

接口鉴权分为两类：

- 商户接口（默认）：需要 `merchant_id`、`timestamp`、`sign`，`access_key` 可选。
- 平台接口（当前仅 `/summarybydaypzid`）：无需 `merchant_id`，但必须提供 `timestamp`、`access_key`、`sign`，且 `access_key` 需为 master key。

以下为商户接口默认公共参数：

| 参数          | 必填 | 说明                                                                   |
|---------------|------|------------------------------------------------------------------------|
| `merchant_id` | ✓    | 商户号（fx_user.userid）                                               |
| `timestamp`   | ✓    | Unix 时间戳（支持 10/13 位），允许 ±300 秒误差                         |
| `access_key`  | 可选 | 全局访问密钥，若匹配则签名使用 master key（推荐调试时启用）            |
| `sign`        | ✓    | 签名字符串                                                             |

### 签名算法

1. 收集所有请求参数（包括业务参数），去除 `sign`。
2. 按键名字典序排序。
3. 过滤值为 `""` 或 `null` 的参数；`0`、`false`、`"0"` 需参与签名。如为数组需 `json_encode`（`JSON_UNESCAPED_UNICODE`）。
4. `key=value` 形式拼接，使用 `&` 连接。
5. 结尾追加 `key={密钥}`，密钥规则：
   - 若提供 `access_key` 且与服务器配置的 master key 相同，则使用 master key；
   - 否则使用商户私钥 `miyao`。
6. 对整个字符串做 `md5`，并转大写即为 `sign`。

---

## 2. 接口总览

| 功能                  | 路径                            |
|-----------------------|---------------------------------|
| 查询账户余额          | `/balance`                      |
| 查询费率              | `/rates`                        |
| 订单列表              | `/orders`                       |
| 经营概览              | `/stats`                        |
| 订单详情              | `/orderdetail`                  |
| 查询订单渠道配置      | `/findpzidbyorder`              |
| 手动补单              | `/manualcompleteorder`          |
| 手动撤单              | `/manualcancelorder`            |
| 回调日志              | `/notifylogs`                   |
| 按日汇总              | `/summarybyday`                 |
| 按日×PZID 平台汇总    | `/summarybydaypzid`             |
| 按日×通道汇总         | `/summarybydaychannel`          |
| 资金流水              | `/moneylogs`                    |
| 提现申请              | `/sendmoney`                    |
| 提现列表              | `/withdrawlist`                 |
| 提现详情              | `/withdrawdetail`               |
| 通道状态              | `/channelstatus`                |
| 费用报表              | `/feestatement`                 |
| 风险概览              | `/riskstats`                    |
| 手工调账记录          | `/manualadjustments`            |
| **模拟下单（新增）**  | `/createorder`                  |

以下接口均默认需要商户公共参数（`merchant_id`、`timestamp`、`access_key`、`sign`），仅列出额外的业务参数。
例外：`/summarybydaypzid` 走平台级鉴权，可不传 `merchant_id`。

---

## 3. 余额查询 `/balance`

**说明**：查询账户余额、待提现金额等。

**业务参数**（可选）：

| 参数            | 说明                                               |
|-----------------|----------------------------------------------------|
| `history_days`  | 历史余额回溯天数，默认 1，最小 1，最大 365         |

**返回字段**：

| 字段               | 说明           |
|--------------------|----------------|
| `merchant_id`      | 商户号         |
| `balance`          | 可用余额       |
| `pending_withdraw` | 待提现金额     |
| `currency`         | 币种（固定 CNY）|
| `updated_at`       | 更新时间       |
| `history_days`     | 本次查询使用的历史余额回溯天数 |
| `history_balance`  | 指定 `history_days` 前的期末余额 |

**返回示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "merchant_id": "2024164",
    "balance": "8848.90",
    "pending_withdraw": "4768892.65",
    "currency": "CNY",
    "updated_at": "2025-11-17 02:26:19",
    "history_days": 1,
    "history_balance": "8848.90"
  }
}
```

---

## 4. 费率查询 `/rates`

**说明**：获取商户开通的通道及费率设置。

**返回字段**：

`items` 数组，每项含：

| 字段               | 说明                                 |
|--------------------|--------------------------------------|
| `channel_code`     | 通道代码                             |
| `channel_name`     | 通道名称                             |
| `status`           | `enabled` / `disabled`               |
| `merchant_switch`  | 商户开关                             |
| `system_switch`    | 系统开关                             |
| `rate` / `rate_display` | 费率数值及展示形式             |
| `user_defined_rate`| 是否为自定义费率                     |

**返回示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "merchant_id": "2024164",
    "items": [
      {
        "channel_code": "wxhftest",
        "channel_name": "微信话费测试专用",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "zfbhftest",
        "channel_name": "支付宝话费测试专用",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "cjwxhf",
        "channel_name": "微信话费慢充",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "cjzfbhf",
        "channel_name": "支付宝话费慢充",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "wxwthf",
        "channel_name": "微信网厅话费",
        "status": "enabled",
        "merchant_switch": true,
        "system_switch": true,
        "rate": 11.5,
        "rate_display": "11.5%",
        "user_defined_rate": true
      },
      {
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "status": "enabled",
        "merchant_switch": true,
        "system_switch": true,
        "rate": 11.5,
        "rate_display": "11.5%",
        "user_defined_rate": true
      },
      {
        "channel_code": "qqhf",
        "channel_name": "QQ话费",
        "status": "enabled",
        "merchant_switch": true,
        "system_switch": true,
        "rate": 10.5,
        "rate_display": "10.5%",
        "user_defined_rate": true
      },
      {
        "channel_code": "tbsqhf",
        "channel_name": "淘宝授权话费",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "ydwtzfb",
        "channel_name": "移动网厅支付宝",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "ysfhf",
        "channel_name": "云闪付话费",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "ncys",
        "channel_name": "内层原生",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "zfbqjd",
        "channel_name": "支付宝旗舰店",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "cwtlt",
        "channel_name": "纯联通",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": false,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      },
      {
        "channel_code": "swzfb",
        "channel_name": "三网支付宝",
        "status": "enabled",
        "merchant_switch": true,
        "system_switch": true,
        "rate": 9,
        "rate_display": "9%",
        "user_defined_rate": true
      },
      {
        "channel_code": "zft",
        "channel_name": "直付通",
        "status": "disabled",
        "merchant_switch": false,
        "system_switch": true,
        "rate": 0,
        "rate_display": "-",
        "user_defined_rate": false
      }
    ]
  }
}
```

---

## 5. 订单列表 `/orders`

**业务参数**（均可选）：

| 参数                  | 说明                                            |
|-----------------------|-------------------------------------------------|
| `status`              | 0 未支付（含手动撤单后回写状态），1 成功，2 扣量 |
| `merchant_order_no`   | 商户订单号                                     |
| `platform_order_no`   | 平台订单号                                     |
| `channel_code`        | 通道代码                                       |
| `start_time` ~ `end_time` | 创建时间区间                            |
| `pay_start` ~ `pay_end`   | 支付时间区间                            |
| `min_amount` / `max_amount` | 金额区间                             |
| `page` / `page_size`  | 分页（默认 1/20，`page_size ≤ 100`）         |

**返回字段**：

`items` 数组：含订单金额、支付状态、通知状态、通道信息、时间戳、商品信息等。  
`summary`：总金额、商户实收、代理收益汇总。

**返回示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "merchant_id": "2024164",
    "page": 1,
    "page_size": 20,
    "total": 112002,
    "total_pages": 5601,
    "items": [
      {
        "merchant_order_no": "210127021460157465303",
        "platform_order_no": "H25111623144996156788",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 23:14:50",
        "paid_at": "2025-11-16 23:16:49",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460148520133",
        "platform_order_no": "H25111622264574041855",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 22:26:46",
        "paid_at": "2025-11-16 22:28:39",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460139249826",
        "platform_order_no": "H25111621404912637323",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 21:40:49",
        "paid_at": "2025-11-16 21:42:42",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460113029217",
        "platform_order_no": "k1989996777515327489",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 17:59:38",
        "paid_at": "2025-11-16 18:00:00",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460080766922",
        "platform_order_no": "k1989913951801516034",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 12:30:31",
        "paid_at": "2025-11-16 12:31:20",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460077748244",
        "platform_order_no": "k1989906556048187393",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 12:01:08",
        "paid_at": "2025-11-16 12:02:20",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460077204545",
        "platform_order_no": "k1989904589058023425",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 11:53:19",
        "paid_at": "2025-11-16 11:53:55",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460075747233",
        "platform_order_no": "k1989900525578559490",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 11:37:10",
        "paid_at": "2025-11-16 11:37:55",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460067825479",
        "platform_order_no": "k1989876673238605826",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 10:02:23",
        "paid_at": "2025-11-16 10:23:41",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460063723786",
        "platform_order_no": "k1989863960932327425",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "200.00",
        "merchant_income": "177.00",
        "agent_income": "8.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 09:11:52",
        "paid_at": "2025-11-16 09:12:35",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460062852675",
        "platform_order_no": "P1989861721992421378",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 09:02:59",
        "paid_at": "2025-11-16 09:11:07",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021460032960977",
        "platform_order_no": "H25111602595299957070",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-16 02:59:53",
        "paid_at": "2025-11-16 03:01:43",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450160511951",
        "platform_order_no": "k1989718420856840193",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 23:33:33",
        "paid_at": "2025-11-15 23:34:10",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450157842951",
        "platform_order_no": "H25111523211657620044",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 23:21:16",
        "paid_at": "2025-11-15 23:23:21",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450155360010",
        "platform_order_no": "k1989712621598220290",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 23:10:30",
        "paid_at": "2025-11-15 23:11:40",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450147163052",
        "platform_order_no": "k1989700914830188546",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 22:23:59",
        "paid_at": "2025-11-15 22:24:30",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450146732934",
        "platform_order_no": "H25111522215984215590",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 22:21:59",
        "paid_at": "2025-11-15 22:24:00",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450140971080",
        "platform_order_no": "k1989693069707517954",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 21:52:49",
        "paid_at": "2025-11-15 21:53:50",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450136450545",
        "platform_order_no": "k1989686128876789761",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 21:25:14",
        "paid_at": "2025-11-15 21:26:04",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      },
      {
        "merchant_order_no": "210127021450123872921",
        "platform_order_no": "k1989665416023711745",
        "status_code": 1,
        "status": "paid",
        "status_text": "支付成功",
        "notify_status_code": 2,
        "notify_status": "success",
        "notify_status_text": "通知成功",
        "total_amount": "100.00",
        "merchant_income": "88.50",
        "agent_income": "4.00",
        "channel_code": "zfbwthf",
        "channel_name": "支付宝网厅话费",
        "created_at": "2025-11-15 20:02:56",
        "paid_at": "2025-11-15 20:03:16",
        "product_title": "desc",
        "attach": "",
        "notify_url": "https://example.com",
        "return_url": "https://example.com"
      }
    ],
    "summary": {
      "gross_amount": "13378250.00",
      "merchant_income": "11839791.25",
      "agent_income": "60140.00"
    }
  }
}
```

---

## 6. 经营概览 `/stats`

**说明**：返回今日、昨日、历史、待支付等核心指标。

**返回字段**：

| 字段       | 说明                                 |
|------------|--------------------------------------|
| `today`    | 今日订单数/金额/实收/代理收益         |
| `yesterday`| 昨日指标                             |
| `total`    | 历史累计                             |
| `pending`  | 待支付订单数及金额                   |

**返回示例**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "merchant_id": "2024164",
    "today": {
      "order_count": 0,
      "gross_amount": "0.00",
      "merchant_income": "0.00",
      "agent_income": "0.00"
    },
    "yesterday": {
      "order_count": 12,
      "gross_amount": "1300.00",
      "merchant_income": "1150.50",
      "agent_income": "52.00"
    },
    "total": {
      "order_count": 19301,
      "gross_amount": "2220500.00",
      "merchant_income": "1965142.50",
      "agent_income": "60140.00"
    },
    "pending": {
      "order_count": 92701,
      "gross_amount": "11157750.00"
    }
  }
}
```

---

## 7. 订单详情 `/orderdetail`

**参数**（二选一）：
| 参数                | 说明                  |
|---------------------|-----------------------|
| `merchant_order_no` | 商户订单号            |
| `platform_order_no` | 平台订单号            |

`merchant_order_no` 与 `platform_order_no` 至少提供一个；可选参数 `with_notify_logs` 传 `1`/`true` 时会额外返回通知日志。

**返回**：
- `order` 基础信息 + `extended` 扩展字段（订单 ID、渠道费、扣量状态等）
- `notify_logs`（可选）

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "merchant_order_no": "210127021460067825479",
        "order": {
            "merchant_order_no": "210127021460067825479",
            "platform_order_no": "k1989876673238605826",
            "status_code": 1,
            "status": "paid",
            "status_text": "支付成功",
            "notify_status_code": 2,
            "notify_status": "success",
            "notify_status_text": "通知成功",
            "total_amount": "100.00",
            "merchant_income": "88.50",
            "agent_income": "4.00",
            "channel_code": "zfbwthf",
            "channel_name": "支付宝网厅话费",
            "created_at": "2025-11-16 10:02:23",
            "paid_at": "2025-11-16 10:23:41",
            "product_title": "desc",
            "attach": "",
            "notify_url": "https://example.com",
            "return_url": "https://example.com",
            "extended": {
                "order_id": 32179372,
                "merchant_order_no_full": "2024164210127021460067825479",
                "raw_channel_code": "zfbwthf",
                "request_ip": "223.104.76.196",
                "notify_status_code": 2,
                "upstream_order_no": "k1989876673238605826",
                "upstream_channel": "",
                "client_attach": {
                    "fxdesc": "desc",
                    "fxattch": null,
                    "fxnotifyurl": "https://example.com",
                    "fxbackurl": "https://example.com",
                    "fxip": "223.104.76.196",
                    "fxbankcode": null,
                    "fxfs": null,
                    "fxuserid": "943088327",
                    "fxnotifystyle": "2",
                    "fxsignstring": ""
                },
                "status_code": 1,
                "notify_retries": 1,
                "deduct_status": 2,
                "created_at": "2025-11-16 10:02:23",
                "paid_at": "2025-11-16 10:23:41",
                "channel_fee": "7.50",
                "platform_profit": "7.00"
            }
        }
    }
}
```

**返回示例 with_notify_logs**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "merchant_order_no": "210127021460067825479",
        "order": {
            "merchant_order_no": "210127021460067825479",
            "platform_order_no": "k1989876673238605826",
            "status_code": 1,
            "status": "paid",
            "status_text": "支付成功",
            "notify_status_code": 2,
            "notify_status": "success",
            "notify_status_text": "通知成功",
            "total_amount": "100.00",
            "merchant_income": "88.50",
            "agent_income": "4.00",
            "channel_code": "zfbwthf",
            "channel_name": "支付宝网厅话费",
            "created_at": "2025-11-16 10:02:23",
            "paid_at": "2025-11-16 10:23:41",
            "product_title": "desc",
            "attach": "",
            "notify_url": "https://example.com",
            "return_url": "https://example.com",
            "extended": {
                "order_id": 32179372,
                "merchant_order_no_full": "2024164210127021460067825479",
                "raw_channel_code": "zfbwthf",
                "request_ip": "223.104.76.196",
                "notify_status_code": 2,
                "upstream_order_no": "k1989876673238605826",
                "upstream_channel": "",
                "client_attach": {
                    "fxdesc": "desc",
                    "fxattch": null,
                    "fxnotifyurl": "https://example.com",
                    "fxbackurl": "https://example.com",
                    "fxip": "223.104.76.196",
                    "fxbankcode": null,
                    "fxfs": null,
                    "fxuserid": "943088327",
                    "fxnotifystyle": "2",
                    "fxsignstring": ""
                },
                "status_code": 1,
                "notify_retries": 1,
                "deduct_status": 2,
                "created_at": "2025-11-16 10:02:23",
                "paid_at": "2025-11-16 10:23:41",
                "channel_fee": "7.50",
                "platform_profit": "7.00"
            }
        },
        "notify_logs": [
            {
                "log_id": 7825692,
                "callback_url": "https://example.com",
                "request_payload": {
                    "fxid": "2024164",
                    "fxddh": "210127021460067825479",
                    "fxorder": "k1989876673238605826",
                    "fxdesc": "desc",
                    "fxfee": "100.00",
                    "fxattch": null,
                    "fxstatus": 1,
                    "fxtime": 1763259821,
                    "fxsign": "c2c7313b702c21a8f0bd4fd5171a6748"
                },
                "response_body": "success",
                "status": "",
                "created_at": "2025-11-16 10:23:43"
            }
        ]
    }
}
```

### 7.1 手动补单 `/manualcompleteorder`

**说明**：将未支付/扣量订单标记为成功，效果等同后台“手动补单”。

**业务参数**：

| 参数                | 必填 | 说明                                         |
|---------------------|------|----------------------------------------------|
| `merchant_order_no` | 否   | 商户订单号                                   |
| `platform_order_no` | 否   | 平台订单号                                   |
| `amount`            | 否   | 金额校验（单位元，传入时需与订单金额一致）   |

`merchant_order_no` 与 `platform_order_no` 至少提供一个。

**返回**：

- `order`：补单后的订单数据（同订单列表里的字段）。
- 若订单已成功，会直接返回当前状态。

> 注意：仅当前状态为 `status_code=0/2` 的订单可补单（`status_code=0` 可能包含手动撤单后回写状态），操作会触发平台内部的资金结算逻辑。

**返回示例**：

```json
{
    "code": 0,
    "message": "订单已是成功状态。",
    "data": {
        "merchant_id": "2023100",
        "merchant_order_no": "202511170232492682",
        "order": {
            "merchant_order_no": "202511170232492682",
            "platform_order_no": "api20251117023353341217",
            "status_code": 1,
            "status": "paid",
            "status_text": "支付成功",
            "notify_status_code": 1,
            "notify_status": "failed",
            "notify_status_text": "通知失败",
            "total_amount": "50.00",
            "merchant_income": "45.00",
            "agent_income": "0.00",
            "channel_code": "wxhftest",
            "channel_name": "微信话费测试专用",
            "created_at": "2025-11-17 02:32:49",
            "paid_at": "2025-11-17 02:33:53",
            "product_title": "商品支付",
            "attach": "",
            "notify_url": "https://example.com",
            "return_url": "https://example.com"
        }
    }
}
```

### 7.2 手动撤单 `/manualcancelorder`

**说明**：撤销未支付 / 已支付 / 扣量订单，效果与后台“手动退单”一致（内部适配状态 0、1、2）。若订单原状态为已支付，系统会先执行资金回退，再回写为 `status_code=0`（`pending`）。

**业务参数**：

| 参数                | 必填 | 说明                                       |
|---------------------|------|--------------------------------------------|
| `merchant_order_no` | 否   | 商户订单号                                |
| `platform_order_no` | 否   | 平台订单号                                |
| `amount`            | 否   | 金额校验（单位元，传入时需与订单金额一致）|

二者至少提供一个。

**返回**：撤单后的订单数据（字段同 `/orders`）。若订单号不存在或金额不匹配会返回错误提示。

> 说明：当前状态码仅返回 `0/1/2`，手动撤单后的订单会表现为 `0`。如需区分“自然未支付”与“手动撤单”，请结合后台操作记录或资金流水判断。

**返回示例**：

```json
{
    "code": 0,
    "message": "撤单成功",
    "data": {
        "merchant_id": "2023100",
        "merchant_order_no": "202511170232492682",
        "order": {
            "merchant_order_no": "202511170232492682",
            "platform_order_no": "",
            "status_code": 0,
            "status": "pending",
            "status_text": "未支付",
            "notify_status_code": 0,
            "notify_status": "pending",
            "notify_status_text": "未通知",
            "total_amount": "50.00",
            "merchant_income": "45.00",
            "agent_income": "0.00",
            "channel_code": "wxhftest",
            "channel_name": "微信话费测试专用",
            "created_at": "2025-11-17 02:32:49",
            "paid_at": null,
            "product_title": "商品支付",
            "attach": "",
            "notify_url": "https://example.com",
            "return_url": "https://example.com"
        }
    }
}
```

---

### 7.3 模拟下单 `/createorder`

**说明**：内部调用真实下单流程，快速生成支付链接。

**业务参数**：

| 参数                         | 必填 | 说明                                                                 |
|------------------------------|------|------------------------------------------------------------------------|
| `merchant_order_no` / `order_no` | 否   | 商户订单号（推荐传 `merchant_order_no`，`order_no` 兼容旧参数；未传则自动生成） |
| `amount`         | ✓    | 金额（单位元，保留两位小数）                                          |
| `channel_code`   | 否   | 通道代码（未传自动选用首个可用通道）                                  |
| `notify_url`     | 否   | 异步通知地址，默认 `https://example.com`                  |
| `return_url`     | 否   | 同步回调地址，默认 `https://example.com`                    |
| `description`    | 否   | 商品名称，默认“商品支付”                                              |
| `attach`         | 否   | 附加信息                                                               |
| `client_ip`      | 否   | 客户端 IP，默认获取调用方 IP                                          |
| `order_style`    | 否   | 订单类型：0=普通，1=充值，2=保证金，3=测试，4=商户二维码              |
| `scan_style`     | 否   | `1`/`cashier` 返回收银台链接，其余值默认返回原始链接                  |
| `notify_style`   | 否   | 1=表单，2/`json`=JSON                                                  |
| `bank_code`      | 否   | 银行编码（用于银行类通道）                                            |
| `payment_method` | 否   | 自定义支付方式                                                        |
| `sub_userid`     | 否   | 子商户号                                                              |
| `sign_string`    | 否   | 签名分隔符，默认 `|`                                                  |
| `jump_flag`      | 否   | 跳转标记，透传给收银台行为控制                                        |

**返回字段**：
- `merchant_order_no`：最终订单号
- `payment_url`：支付链接（若返回字符串）
- `payment`：上游原始字段（若返回数组，如二维码地址）
- `order_id`、`order_md5`、`platform_order_no`、`status`

> 注意：接口会真实写入订单，请确保金额、回调地址等信息正确；建议测试时使用较小金额。

**返回示例**：

```json
{
    "code": 0,
    "message": "订单创建成功",
    "data": {
        "merchant_id": "2023100",
        "merchant_order_no": "202511170232492682",
        "amount": "50.00",
        "channel_code": "wxhftest",
        "description": "商品支付",
        "payment": null,
        "payment_url": "https://example.com",
        "order_id": 32180458,
        "platform_order_no": "",
        "order_md5": "5efbeca974a515c5",
        "status": 0
    }
}
```

### 7.4 查询订单所属渠道配置 `/findpzidbyorder`

**说明**：当只需核对订单使用的渠道配置（`pzid`）及通道信息时，可调用此接口快速返回订单与配置的映射关系，避免拉取完整详情。

**参数**（二选一，与 `/orderdetail` 相同）：

| 参数                | 说明                  |
|---------------------|-----------------------|
| `merchant_order_no` | 商户订单号            |
| `platform_order_no` | 平台订单号            |

**返回字段**：

| 字段                    | 说明                                         |
|-------------------------|----------------------------------------------|
| `merchant_id`           | 商户号                                       |
| `merchant_order_no`     | 去前缀后的商户订单号                        |
| `merchant_order_no_full`| 数据库存储的完整订单号（含商户号前缀）      |
| `platform_order_no`     | 平台订单号                                   |
| `order_id`              | 订单主键 ID                                  |
| `pzid` / `pz_name`      | 渠道配置 ID 及名称                           |
| `channel_code` / `channel_name` | 支付通道代码及名称                 |
| `status_code`           | 数字状态：0=未支付（含手动撤单后回写状态），1=成功，2=扣量 |
| `status` / `status_text`| 英文/中文状态描述                           |

> 返回结果中不包含金额、通知等字段，如需更多信息请调用 `/orderdetail`。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2023100",
        "merchant_order_no": "202511170232492682",
        "merchant_order_no_full": "2023100202511170232492682",
        "platform_order_no": "",
        "order_id": 32180458,
        "pzid": 337,
        "pz_name": "华捷四方-网厅微信md5",
        "channel_code": "wxhftest",
        "channel_name": "微信话费测试专用",
        "status_code": 0,
        "status": "pending",
        "status_text": "未支付"
    }
}
```

---

## 8. 回调日志 `/notifylogs`

**参数**：
| 参数                | 说明         |
|---------------------|--------------|
| `merchant_order_no` | 商户订单号   |
| `platform_order_no` | 平台订单号   |
| `page` / `page_size`| 分页参数     |

**返回**：每条记录包含回调 URL、请求报文、响应内容、日志时间等。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "merchant_order_no": "210127021460067825479",
        "page": 1,
        "page_size": 20,
        "total": 1,
        "total_pages": 1,
        "items": [
            {
                "log_id": 7825692,
                "merchant_order_no_full": "2024164210127021460067825479",
                "callback_url": "https://example.com",
                "request_payload": {
                    "fxid": "2024164",
                    "fxddh": "210127021460067825479",
                    "fxorder": "k1989876673238605826",
                    "fxdesc": "desc",
                    "fxfee": "100.00",
                    "fxattch": null,
                    "fxstatus": 1,
                    "fxtime": 1763259821,
                    "fxsign": "c2c7313b702c21a8f0bd4fd5171a6748"
                },
                "response_body": "success",
                "status": "",
                "created_at": "2025-11-16 10:23:43"
            }
        ]
    }
}
```

---

## 9. 按日汇总 `/summarybyday`

**参数**：
| 参数               | 说明        |
|--------------------|-------------|
| `start_time`/`end_time` | 日期区间 |
| `channel_code`     | 限定通道    |

**返回**：按日期聚合的订单笔数、总金额、商户实收、代理收益。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "start_date": "2025-11-11",
        "end_date": "2025-11-17",
        "channel_code": null,
        "items": [
            {
                "date": "2025-11-11",
                "order_count": 14,
                "gross_amount": "1500.00",
                "merchant_income": "1327.50",
                "agent_income": "60.00"
            },
            {
                "date": "2025-11-12",
                "order_count": 11,
                "gross_amount": "1100.00",
                "merchant_income": "973.50",
                "agent_income": "44.00"
            },
            {
                "date": "2025-11-13",
                "order_count": 17,
                "gross_amount": "1900.00",
                "merchant_income": "1681.50",
                "agent_income": "76.00"
            },
            {
                "date": "2025-11-14",
                "order_count": 18,
                "gross_amount": "2100.00",
                "merchant_income": "1858.50",
                "agent_income": "84.00"
            },
            {
                "date": "2025-11-15",
                "order_count": 21,
                "gross_amount": "2100.00",
                "merchant_income": "1858.50",
                "agent_income": "84.00"
            },
            {
                "date": "2025-11-16",
                "order_count": 12,
                "gross_amount": "1300.00",
                "merchant_income": "1150.50",
                "agent_income": "52.00"
            }
        ]
    }
}
```

### 9.1 按日×PZID 平台汇总 `/summarybydaypzid`

**说明**：面向平台视角的按日统计，仅依赖 `pzid` 和时间范围过滤，可查看所有商户在指定上游配置下的支付表现（仅统计 `status>0` 的已支付订单）。该接口走 master key 级别鉴权：无需 `merchant_id`，但仍需传 `timestamp`、`access_key`、`sign`；其中 `access_key` 必须与服务器配置的 master key 匹配，并使用 master key 参与签名。

**参数**：
| 参数               | 必填 | 说明                                                                                  |
|--------------------|------|---------------------------------------------------------------------------------------|
| `pzid`             | ✓    | 上游/渠道配置 ID，正整数（数据库字段 `pzid`）                                         |
| `start_time`/`end_time` 或 `start_date`/`end_date` | 否 | 统计区间；均不传时默认最近 7 天，日期与时间格式与 `/summarybyday` 相同 |

> `start_date`/`end_date` 会在内部转换为 `start_time`/`end_time`，两组参数任选其一即可；若区间为空或开始时间晚于结束时间，将返回「时间范围有误」。

**返回字段**：

| 字段          | 说明                                                                           |
|---------------|--------------------------------------------------------------------------------|
| `pzid`        | 本次查询使用的渠道配置 ID                                                      |
| `pz_name`     | 渠道配置名称（来源于配置表 `pzname`）                                          |
| `start_date`  | 统计起始日期（YYYY-MM-DD）                                                     |
| `end_date`    | 统计结束日期（YYYY-MM-DD）                                                     |
| `items`       | 按日统计数组（仅返回有订单的日期；若无数据则为空数组）                         |

`items` 中每项：

| 字段             | 说明                      |
|------------------|---------------------------|
| `date`           | 日期（YYYY-MM-DD）        |
| `order_count`    | 订单数量                  |
| `gross_amount`   | 订单总金额（字符串，单位元）|
| `merchant_income`| 商户实收（字符串）        |
| `agent_income`   | 代理收益（字符串）        |

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "pzid": 332,
        "pz_name": "华捷四方-1700支付宝md5",
        "start_date": "2025-11-11",
        "end_date": "2025-11-17",
        "items": [
            {
                "date": "2025-11-11",
                "order_count": 30,
                "gross_amount": "2250.00",
                "merchant_income": "2070.00",
                "agent_income": "1.00"
            },
            {
                "date": "2025-11-12",
                "order_count": 48,
                "gross_amount": "3950.00",
                "merchant_income": "3634.00",
                "agent_income": "0.50"
            },
            {
                "date": "2025-11-13",
                "order_count": 49,
                "gross_amount": "3350.00",
                "merchant_income": "3082.00",
                "agent_income": "0.00"
            },
            {
                "date": "2025-11-14",
                "order_count": 47,
                "gross_amount": "3450.00",
                "merchant_income": "3174.00",
                "agent_income": "0.00"
            },
            {
                "date": "2025-11-15",
                "order_count": 19,
                "gross_amount": "1500.00",
                "merchant_income": "1380.00",
                "agent_income": "0.00"
            },
            {
                "date": "2025-11-16",
                "order_count": 18,
                "gross_amount": "1700.00",
                "merchant_income": "1564.00",
                "agent_income": "0.00"
            },
            {
                "date": "2025-11-17",
                "order_count": 1,
                "gross_amount": "50.00",
                "merchant_income": "46.00",
                "agent_income": "0.00"
            }
        ]
    }
}
```

### 9.2 按日×通道汇总 `/summarybydaychannel`

**参数**：
| 参数               | 说明                                   |
|--------------------|----------------------------------------|
| `start_time`/`end_time` | 日期区间                          |
| `channel_codes`    | 指定通道（数组或逗号分隔字符串）      |

**返回**：按日期和通道拆分的统计数据。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "start_date": "2025-11-11",
        "end_date": "2025-11-17",
        "channel_codes": null,
        "items": [
            {
                "date": "2025-11-11",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 14,
                "gross_amount": "1500.00",
                "merchant_income": "1327.50",
                "agent_income": "60.00"
            },
            {
                "date": "2025-11-12",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 11,
                "gross_amount": "1100.00",
                "merchant_income": "973.50",
                "agent_income": "44.00"
            },
            {
                "date": "2025-11-13",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 17,
                "gross_amount": "1900.00",
                "merchant_income": "1681.50",
                "agent_income": "76.00"
            },
            {
                "date": "2025-11-14",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 18,
                "gross_amount": "2100.00",
                "merchant_income": "1858.50",
                "agent_income": "84.00"
            },
            {
                "date": "2025-11-15",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 21,
                "gross_amount": "2100.00",
                "merchant_income": "1858.50",
                "agent_income": "84.00"
            },
            {
                "date": "2025-11-16",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 12,
                "gross_amount": "1300.00",
                "merchant_income": "1150.50",
                "agent_income": "52.00"
            },
            {
                "date": "2025-11-17",
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "order_count": 1,
                "gross_amount": "200.00",
                "merchant_income": "177.00",
                "agent_income": "8.00"
            }
        ]
    }
}
```

---

## 10. 资金流水 `/moneylogs`

**参数**：
| 参数         | 说明                                   |
|--------------|----------------------------------------|
| `style`      | 0=调整 1=充值 2=提现 3=分佣            |
| `start_time`/`end_time` | 时间区间                    |
| `page`/`page_size` | 分页（默认 1/20，最大 200）     |

**返回**：每条记录含变动金额、余额、描述、关联订单等。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "page": 1,
        "page_size": 20,
        "total": 28622,
        "total_pages": 1432,
        "items": [
            {
                "log_id": 7490192,
                "change_amount": "177.00",
                "balance_after": "9025.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额200.00元，到账金额177.00元",
                "order_no": "2024164210127021470028974321",
                "unique_no": "2024164210127021470028974321_bd0",
                "created_at": "2025-11-17 03:35:50"
            },
            {
                "log_id": 7490178,
                "change_amount": "88.50",
                "balance_after": "8848.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460157465303",
                "unique_no": "2024164210127021460157465303_bd0",
                "created_at": "2025-11-16 23:16:49"
            },
            {
                "log_id": 7490172,
                "change_amount": "88.50",
                "balance_after": "8760.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460148520133",
                "unique_no": "2024164210127021460148520133_bd0",
                "created_at": "2025-11-16 22:28:39"
            },
            {
                "log_id": 7490168,
                "change_amount": "88.50",
                "balance_after": "8671.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460139249826",
                "unique_no": "2024164210127021460139249826_bd0",
                "created_at": "2025-11-16 21:42:42"
            },
            {
                "log_id": 7490156,
                "change_amount": "88.50",
                "balance_after": "8583.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460113029217",
                "unique_no": "2024164210127021460113029217_bd0",
                "created_at": "2025-11-16 18:00:00"
            },
            {
                "log_id": 7490142,
                "change_amount": "88.50",
                "balance_after": "8494.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460080766922",
                "unique_no": "2024164210127021460080766922_bd0",
                "created_at": "2025-11-16 12:31:20"
            },
            {
                "log_id": 7490134,
                "change_amount": "88.50",
                "balance_after": "8406.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460077748244",
                "unique_no": "2024164210127021460077748244_bd0",
                "created_at": "2025-11-16 12:02:20"
            },
            {
                "log_id": 7490131,
                "change_amount": "88.50",
                "balance_after": "8317.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460077204545",
                "unique_no": "2024164210127021460077204545_bd0",
                "created_at": "2025-11-16 11:53:55"
            },
            {
                "log_id": 7490127,
                "change_amount": "88.50",
                "balance_after": "8229.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460075747233",
                "unique_no": "2024164210127021460075747233_bd0",
                "created_at": "2025-11-16 11:37:55"
            },
            {
                "log_id": 7490117,
                "change_amount": "88.50",
                "balance_after": "8140.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460067825479",
                "unique_no": "2024164210127021460067825479_bd0",
                "created_at": "2025-11-16 10:23:41"
            },
            {
                "log_id": 7490109,
                "change_amount": "177.00",
                "balance_after": "8052.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额200.00元，到账金额177.00元",
                "order_no": "2024164210127021460063723786",
                "unique_no": "2024164210127021460063723786_bd0",
                "created_at": "2025-11-16 09:12:35"
            },
            {
                "log_id": 7490106,
                "change_amount": "88.50",
                "balance_after": "7875.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460062852675",
                "unique_no": "2024164210127021460062852675_bd0",
                "created_at": "2025-11-16 09:11:07"
            },
            {
                "log_id": 7490099,
                "change_amount": "88.50",
                "balance_after": "7786.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021460032960977",
                "unique_no": "2024164210127021460032960977_bd0",
                "created_at": "2025-11-16 03:01:43"
            },
            {
                "log_id": 7490083,
                "change_amount": "88.50",
                "balance_after": "7698.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450160511951",
                "unique_no": "2024164210127021450160511951_bd0",
                "created_at": "2025-11-15 23:34:10"
            },
            {
                "log_id": 7490077,
                "change_amount": "88.50",
                "balance_after": "7609.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450157842951",
                "unique_no": "2024164210127021450157842951_bd0",
                "created_at": "2025-11-15 23:23:21"
            },
            {
                "log_id": 7490072,
                "change_amount": "88.50",
                "balance_after": "7521.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450155360010",
                "unique_no": "2024164210127021450155360010_bd0",
                "created_at": "2025-11-15 23:11:40"
            },
            {
                "log_id": 7490067,
                "change_amount": "88.50",
                "balance_after": "7432.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450147163052",
                "unique_no": "2024164210127021450147163052_bd0",
                "created_at": "2025-11-15 22:24:30"
            },
            {
                "log_id": 7490063,
                "change_amount": "88.50",
                "balance_after": "7344.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450146732934",
                "unique_no": "2024164210127021450146732934_bd0",
                "created_at": "2025-11-15 22:24:00"
            },
            {
                "log_id": 7490056,
                "change_amount": "88.50",
                "balance_after": "7255.90",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450140971080",
                "unique_no": "2024164210127021450140971080_bd0",
                "created_at": "2025-11-15 21:53:50"
            },
            {
                "log_id": 7490048,
                "change_amount": "88.50",
                "balance_after": "7167.40",
                "type_code": 1,
                "type": "recharge",
                "description": "资金流水记录：订单金额100.00元，到账金额88.50元",
                "order_no": "2024164210127021450136450545",
                "unique_no": "2024164210127021450136450545_bd0",
                "created_at": "2025-11-15 21:26:04"
            }
        ]
    }
}
```

---

## 11. 提现申请 `/sendmoney`

**说明**：发起商户余额提现申请，写入提现记录并同步资金流水。

**业务参数**：

| 参数          | 必填 | 说明                                                         |
|---------------|------|--------------------------------------------------------------|
| `amount`      | ✓    | 提现金额，单位元（可用 `money` 作为同义参数）               |
| `bank_id`     | 否   | 银行卡 ID（已审核的银行卡），未传则自动取首张已审核银行卡   |
| `google_code` | 否   | 谷歌验证码；当后台 `ifggadmindf` 开启时必填（可用 `ggyzm`） |

> 需要同时提交公共签名参数；`timestamp` 建议使用 10 位秒级时间戳。

**返回字段**：

| 字段               | 说明                                   |
|--------------------|----------------------------------------|
| `merchant_id`      | 商户号                                 |
| `withdraw`         | 提现详情对象（字段同 `/withdrawlist` 返回项） |
| `balance_after`    | 扣除申请金额及手续费后的可用余额       |
| `pending_withdraw` | 当前累计待结算金额                     |
| `frozen_today`     | 当日冻结金额（用于计算可提现额度）     |
| `fee`              | 本次提现手续费                         |

**失败场景举例**：金额格式错误、未绑定银行卡、谷歌验证码校验失败、余额不足等，会在 `message` 中返回详细原因。

**返回示例**：

```json
{
    "code": 0,
    "message": "提现申请提交成功",
    "data": {
        "merchant_id": "2023100",
        "withdraw": {
            "withdraw_id": 7490196,
            "withdraw_no": "qzf1763323862322992",
            "amount": "500.00",
            "fee": "0.00",
            "status_code": 0,
            "status": "pending",
            "agentpay_status_code": 0,
            "agentpay_status": "not_submitted",
            "notify_status_code": 0,
            "notify_status": "pending",
            "account_name": "jl888",
            "account_number": "621700******1234",
            "bank_name": "中国建设银行",
            "branch": "深圳福田支行",
            "province": "广东省",
            "city": "深圳市",
            "unionpay_code": "105584000017",
            "created_at": "2025-11-17 04:11:02",
            "extend": []
        },
        "balance_after": "4180.80",
        "pending_withdraw": "1716.20",
        "frozen_today": "0.00",
        "fee": "0.00"
    }
}
```

---

## 12. 提现列表 `/withdrawlist`

**参数**：
| 参数         | 说明                                   |
|--------------|----------------------------------------|
| `status`     | 0申请 1已支付 2冻结 3取消              |
| `start_time`/`end_time` | 时间区间                    |
| `page`/`page_size` | 分页                             |

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "page": 1,
        "page_size": 20,
        "total": 223,
        "total_pages": 12,
        "items": [
            {
                "withdraw_id": 23470,
                "withdraw_no": "qzf1762586848763179",
                "amount": "14500.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-11-08 15:27:28",
                "extend": []
            },
            {
                "withdraw_id": 23454,
                "withdraw_no": "qzf1762001367856204",
                "amount": "10845.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-11-01 20:49:27",
                "extend": []
            },
            {
                "withdraw_id": 23440,
                "withdraw_no": "qzf1761822982396290",
                "amount": "21750.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-30 19:16:22",
                "extend": []
            },
            {
                "withdraw_id": 23433,
                "withdraw_no": "qzf1761481634186409",
                "amount": "21780.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-26 20:27:14",
                "extend": []
            },
            {
                "withdraw_id": 23425,
                "withdraw_no": "qzf1761188138134106",
                "amount": "10905.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-23 10:55:38",
                "extend": []
            },
            {
                "withdraw_id": 23422,
                "withdraw_no": "qzf1760971603963058",
                "amount": "7270.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-20 22:46:43",
                "extend": []
            },
            {
                "withdraw_id": 23414,
                "withdraw_no": "qzf1760618049604193",
                "amount": "14520.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-16 20:34:09",
                "extend": []
            },
            {
                "withdraw_id": 23397,
                "withdraw_no": "qzf1759772849727830",
                "amount": "14520.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-07 01:47:29",
                "extend": []
            },
            {
                "withdraw_id": 23365,
                "withdraw_no": "qzf1759323110134603",
                "amount": "21750.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-10-01 20:51:50",
                "extend": []
            },
            {
                "withdraw_id": 23351,
                "withdraw_no": "qzf1759153156475832",
                "amount": "14500.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-29 21:39:16",
                "extend": []
            },
            {
                "withdraw_id": 23325,
                "withdraw_no": "qzf1758706339308659",
                "amount": "21690.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-24 17:32:19",
                "extend": []
            },
            {
                "withdraw_id": 23299,
                "withdraw_no": "qzf1758108110292787",
                "amount": "14440.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-17 19:21:50",
                "extend": []
            },
            {
                "withdraw_id": 23275,
                "withdraw_no": "qzf1757761859699841",
                "amount": "10845.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-13 19:10:59",
                "extend": []
            },
            {
                "withdraw_id": 23253,
                "withdraw_no": "qzf1757546544700951",
                "amount": "4344.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-11 07:22:24",
                "extend": []
            },
            {
                "withdraw_id": 23198,
                "withdraw_no": "qzf1757046069213329",
                "amount": "14500.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-05 12:21:09",
                "extend": []
            },
            {
                "withdraw_id": 23159,
                "withdraw_no": "qzf1756812722180255",
                "amount": "7260.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-09-02 19:32:02",
                "extend": []
            },
            {
                "withdraw_id": 23127,
                "withdraw_no": "qzf1756625055543975",
                "amount": "14560.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-08-31 15:24:15",
                "extend": []
            },
            {
                "withdraw_id": 23032,
                "withdraw_no": "qzf1755953199916042",
                "amount": "14540.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-08-23 20:46:39",
                "extend": []
            },
            {
                "withdraw_id": 22986,
                "withdraw_no": "qzf1755677696144092",
                "amount": "20567.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-08-20 16:14:56",
                "extend": []
            },
            {
                "withdraw_id": 22925,
                "withdraw_no": "qzf1755237333737740",
                "amount": "14540.00",
                "fee": "0.00",
                "status_code": 1,
                "status": "paid",
                "agentpay_status_code": 0,
                "agentpay_status": "not_submitted",
                "notify_status_code": 0,
                "notify_status": "pending",
                "account_name": "jl888",
                "account_number": "系统扣除",
                "bank_name": "系统扣除",
                "branch": null,
                "province": null,
                "city": null,
                "unionpay_code": null,
                "created_at": "2025-08-15 13:55:33",
                "extend": []
            }
        ]
    }
}
```

---

## 13. 提现详情 `/withdrawdetail`

**参数**：
| 参数          | 说明                                   |
|---------------|----------------------------------------|
| `withdraw_no` | 提现订单号（推荐）                     |
| `withdraw_id` | 提现表主键 ID                          |
| `order_no`    | 兼容旧参数，等价于 `withdraw_id`       |

**返回**：提现基础信息、回调日志、上游代付记录。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "withdraw": {
            "withdraw_id": 23470,
            "withdraw_no": "qzf1762586848763179",
            "amount": "14500.00",
            "fee": "0.00",
            "status_code": 1,
            "status": "paid",
            "agentpay_status_code": 0,
            "agentpay_status": "not_submitted",
            "notify_status_code": 0,
            "notify_status": "pending",
            "account_name": "jl888",
            "account_number": "系统扣除",
            "bank_name": "系统扣除",
            "branch": null,
            "province": null,
            "city": null,
            "unionpay_code": null,
            "created_at": "2025-11-08 15:27:28",
            "extend": [],
            "notify_logs": [],
            "upstream_records": []
        }
    }
}
```

---

## 14. 通道状态 `/channelstatus`

**说明**：查询商户通道开关、限额、费率等。

**返回字段**：`channel_code`、`system_enabled`、`merchant_enabled`、`rate`、`min_amount`、`max_amount`、`daily_quota`、`daily_used`、`last_used_at` 等。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "items": [
            {
                "channel_code": "wxhftest",
                "channel_name": "微信话费测试专用",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": false,
                "min_amount": "10.00",
                "max_amount": "500.00",
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "zfbhftest",
                "channel_name": "支付宝话费测试专用",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": false,
                "min_amount": "100.00",
                "max_amount": "10000.00",
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "cjwxhf",
                "channel_name": "微信话费慢充",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "cjzfbhf",
                "channel_name": "支付宝话费慢充",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "wxwthf",
                "channel_name": "微信网厅话费",
                "system_enabled": true,
                "merchant_enabled": true,
                "rate": 11.5,
                "user_defined_rate": true,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "zfbwthf",
                "channel_name": "支付宝网厅话费",
                "system_enabled": true,
                "merchant_enabled": true,
                "rate": 11.5,
                "user_defined_rate": true,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "qqhf",
                "channel_name": "QQ话费",
                "system_enabled": true,
                "merchant_enabled": true,
                "rate": 10.5,
                "user_defined_rate": true,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "tbsqhf",
                "channel_name": "淘宝授权话费",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "ydwtzfb",
                "channel_name": "移动网厅支付宝",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "ysfhf",
                "channel_name": "云闪付话费",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": false,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "ncys",
                "channel_name": "内层原生",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": false,
                "min_amount": "10.00",
                "max_amount": "500.00",
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "zfbqjd",
                "channel_name": "支付宝旗舰店",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": false,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "cwtlt",
                "channel_name": "纯联通",
                "system_enabled": false,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": false,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "swzfb",
                "channel_name": "三网支付宝",
                "system_enabled": true,
                "merchant_enabled": true,
                "rate": 9,
                "user_defined_rate": true,
                "is_round_robin": true,
                "min_amount": "0.00",
                "max_amount": null,
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            },
            {
                "channel_code": "zft",
                "channel_name": "直付通",
                "system_enabled": true,
                "merchant_enabled": false,
                "rate": 0,
                "user_defined_rate": false,
                "is_round_robin": true,
                "min_amount": "100.00",
                "max_amount": "10000.00",
                "daily_quota": null,
                "daily_used": null,
                "last_used_at": null
            }
        ]
    }
}
```

---

## 15. 费用报表 `/feestatement`

**参数**：
| 参数         | 说明                                   |
|--------------|----------------------------------------|
| `start_time`/`end_time` | 时间区间，默认近 30 天        |

**返回**：订单数、交易总额、商户实收、平台收入、平台手续费等。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "start_date": "2025-10-19",
        "end_date": "2025-11-17",
        "order_count": 875,
        "gross_amount": "104700.00",
        "merchant_income": "92659.50",
        "agent_income": "4188.00",
        "platform_income": "7329.00",
        "platform_fee": "7852.50"
    }
}
```

---

## 16. 风险概览 `/riskstats`

**参数**：
| 参数          | 说明                          |
|---------------|-------------------------------|
| `high_amount` | 高额订单阈值（默认 1000 元） |

**返回**：超过 30 分钟未支付订单数、高额订单数、通知失败数、扣量订单数等。

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "timestamp": "2025-11-17 04:23:37",
        "pending_orders_over_30m": 92708,
        "high_value_orders_today": 0,
        "failed_notifications": 17,
        "withheld_orders": 0
    }
}
```

---

## 17. 手工调账 `/manualadjustments`

**说明**：查询疑似人工调整的资金流水。

**参数**：
| 参数         | 说明                                   |
|--------------|----------------------------------------|
| `page`/`page_size` | 分页                             |
| `start_time`/`end_time` | 时间区间                    |

**返回示例**：

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "merchant_id": "2024164",
        "page": 1,
        "page_size": 20,
        "total": 0,
        "total_pages": 0,
        "items": []
    }
}
```

---

## 18. 错误码（常见 `message`）

| 提示                        | 说明                                 |
|-----------------------------|--------------------------------------|
| `签名错误`                  | `sign` 校验失败                      |
| `timestamp已过期`           | 时间戳超出允许范围                   |
| `商户号不存在或已停用`      | 商户状态异常                         |
| `notify_url 无效`           | 参数校验失败                         |
| `没有对应金额`              | 通道金额限制或未配置                 |
| `系统繁忙，请稍后重试。`    | 系统内部异常                         |

---

## 19. 调试建议

1. 调试时务必更新 `timestamp` 并重新计算签名。
2. 若提示 `签名错误`，请检查参数排序、是否遗漏业务参数、是否使用 master key。
3. `/createorder` 会真实写库，请使用合适回调地址和金额（建议小额测试）。
4. 对接过程中如需 SDK 或进一步支持，请联系技术团队。
