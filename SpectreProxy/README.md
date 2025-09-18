# SpectreProxy

![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)![Language](https://img.shields.io/badge/language-JavaScript-orange.svg)![Platform](https://img.shields.io/badge/platform-Cloudflare%20Workers-red)

一个基于 Cloudflare Workers 和原生 TCP Socket 的高级智能代理网关。它通过完全控制请求流，解决了原生 `fetch` API 存在的隐私泄露问题，并提供了灵活的回退策略、智能路由和多协议支持，专为需要高稳定性、隐私保护和复杂网络访问的场景设计。

## 目录
- [免责声明](#免责声明)
- [项目背景](#项目背景)
- [核心原理与特性](#核心原理与特性)
  - [核心原理](#核心原理)
  - [主要特性](#主要特性-1)
- [部署指南](#部署指南)
  - [版本选择](#版本选择)
  - [部署步骤](#部署步骤)
  - [配置环境变量](#配置环境变量)
- [环境变量配置](#环境变量配置-1)
  - [通用配置 (single.js & AIGateway)](#通用配置-singlejs--aigateway)
  - [AIGateway 专属配置](#aigateway-专属配置)
  - [single.js 专属配置](#singlejs-专属配置)
- [使用方法](#使用方法)
  - [AIGateway (AI 代理优化版)](#aigateway-ai-代理优化版)
  - [single.js (通用版)](#singlejs-通用版)
- [兼容性测试](#兼容性测试)
- [开发指南](#开发指南)
  - [项目结构](#项目结构)
  - [第三方HTTP代理](#第三方http代理)
  - [其他云平台兼容代理](#其他云平台兼容代理)
- [变更记录 (Changelog)](#变更记录-changelog)
- [License](#license)

## 免责声明

本项目是一个用于学习和理解 Cloudflare Worker 机制的示例，仅供个人学习和研究网络技术之用，**严禁用于任何非法目的**。

任何使用者在使用本项目时，均应遵守其所在国家或地区的法律法规。对于任何因使用或滥用本项目而导致的任何直接或间接的法律责任和风险，**均由使用者本人承担**，作者及贡献者不对任何第三方使用本项目进行的任何非法活动及其造成的任何损害承担责任。

如果您开始使用本项目，即表示您已充分理解并同意上述条款。

## 项目背景

Cloudflare Workers 的 `fetch` API 是一个功能强大的工具，可以轻松实现反向代理。然而，它会自动在请求中添加特定的 `CF-*` 请求头，例如：

- `cf-connecting-ip`：暴露发起请求用户的真实 IP 地址。
- `cf-ipcountry`：暴露用户所在的国家/地区。
- `cf-worker`：明确标识该请求经过了 Cloudflare Workers。

这些请求头无法通过常规方式移除，会导致以下问题：

1.  **隐私泄露**：用户的真实 IP 被暴露给目标服务器。
2.  **访问限制**：部分对国家/地区有限制的服务（如 OpenAI, Claude）会因 `cf-country` 而拒绝访问。
3.  **代理暴露**：目标网站可以轻易识别出请求来自 Cloudflare Workers，甚至可能因此封禁域名。

![与fetch api对比](https://github.com/XyzenSun/SpectreProxy/raw/main/img/img_contrast.jpg)

## 核心原理与特性

为解决上述问题，SpectreProxy 采用了更底层的解决方案。

### 核心原理

项目通过 Cloudflare Workers 的 **原生 TCP Socket API** (`connect()`) 直接构建 HTTP/1.1、WebSocket 及 DNS 请求。这种方式可以完全控制请求的每一个字节，从根源上避免了 `fetch` API 自动添加的、可能泄露隐私的请求头。

### 主要特性

- **隐私保护**：通过原生 Socket 隐藏真实 IP 和代理痕迹，保护用户隐私。
- **智能路由与回退**：
    - **`AIGateway`**: 可根据目标域名自动选择 **SOCKS5 代理** 或 **直连**，并支持直连失败后自动回退到 SOCKS5。
    - **`single.js`**: 支持 `socket`, `fetch`, `socks5` 等多种策略，并可在主策略失败时切换到备用策略。
- **多协议支持 (`single.js`)**：原生支持 HTTP/S、WebSocket (WSS)，以及 DNS-over-HTTPS (DoH) / DNS-over-TLS (DoT) 查询。
- **高级请求伪装 (`AIGateway`)**: 支持随机化 `User-Agent` 和 `Accept-Language`，并能根据原始请求的 UA 类型（移动/桌面）智能匹配。
- **高可用性 (`AIGateway`)**:
    - 支持从多个 API 地址动态获取 SOCKS5 代理并随机选用。
    - 对幂等请求在遇到 `5xx` 错误时自动进行指数退避重试。
- **易于部署**：提供单文件 Worker 脚本，无需复杂的外部依赖。

## 部署指南

### 版本选择

项目提供两个版本，请根据需求选择：

- **`aigateway.js`**: **推荐用于代理 AI API**。经过深度优化，内置智能路由、SOCKS5 代理、自动重试和头部伪装，性能和稳定性更佳。
- **`single.js`**: **通用版本**。支持 HTTP, WebSocket, DoH/DoT 等多种协议，配置灵活，适合更广泛的代理场景。

### 部署步骤

1.  **准备 Cloudflare 环境**：确保你拥有一个 Cloudflare 账号，并已激活 Workers 服务。
2.  **创建 Worker**：在 Cloudflare 控制面板中，`计算(Workers)`->`Workers 和 Pages` > `创建`->`从 Hello World! 开始`->`部署` -> `继续处理项目` -> `编辑代码`。
3.  **复制代码**：复制所选版本（`aigateway.js` 或 `single.js`）的全部内容，粘贴到 `worker.js` 中，并点击 **部署**。
4.  **配置环境变量**：在 Worker 的 `设置` > `变量` 页面中，根据下文说明添加环境变量以完成配置。

## 环境变量配置

### 通用配置 (single.js & AIGateway)

| 环境变量          | 说明                                                         |
| ----------------- | ------------------------------------------------------------ |
| `AUTH_TOKEN`      | **访问代理所需的认证密钥，务必修改为你自己的强密码**。默认值为 `"your-auth-token"` 或 `"defaulttoken"`。 |
| `DEFAULT_DST_URL` | 当不指定目标时，默认访问的 URL。                             |
| `DEBUG_MODE`      | 是否开启调试模式，开启后会在控制台输出详细日志。建议生产环境设为 `false`。 |

### AIGateway 专属配置

| 环境变量                               | 默认值  | 说明                                    |
| -------------------------------------- | ------- | --------------------------------------- |
| `ENABLE_UA_RANDOMIZATION`              | `true`  | 是否启用随机 User-Agent 功能。          |
| `ENABLE_ACCEPT_LANGUAGE_RANDOMIZATION` | `false` | 是否启用随机 `Accept-Language` 功能。   |
| `ENABLE_SOCKS5_FALLBACK`               | `true`  | 当直连失败时，是否自动尝试使用 SOCKS5。 |

**提示**: `AIGateway` 的高级路由和 SOCKS5 源在代码内配置

`SOCKS5_API_URLS`:表示通过Get请求的SOCKS5API路径，如果需要鉴权，修改`parseSocks5Proxy()`中

 ` const response = await fetch(randomApiUrl, { method: 'GET' });`

`HOST_REQUEST_CONFIG`：通过HOST进行分流，nativeFetch表示使用socket手动实现的http请求，失败了会自动用socks5回退，socks5表示直接用socks5进行代理请求，适用于使用了cloudflare的网站，避免重试，提升性能。

`URL_PRESETS`：URL路径映射，例如`"gemini", "https://generativelanguage.googleapis.com"` 相当于把`https://your-worker.workers.dev/your-token/gemini` 映射到 `https://generativelanguage.googleapis.com` ; 请求` https://your-worker.workers.dev/your-token/gemini `相当于请求`https://your-worker.workers.dev/your-token/https://generativelanguage.googleapis.com`

### single.js 专属配置

| 环境变量                  | 默认值         | 说明                                                         |
| ------------------------- | -------------- | ------------------------------------------------------------ |
| `PROXY_STRATEGY`          | `"socket"`     | **主代理策略**。可选值: `socket`, `fetch`, `socks5` 等。     |
| `FALLBACK_PROXY_STRATEGY` | `"fetch"`      | **回退代理策略**。当主策略失败时启用。                       |
| `SOCKS5_ADDRESS`          | `""`           | SOCKS5 代理地址，格式为 `host:port`或`user:password@host:port`。 |
| `THIRD_PARTY_PROXY_URL`   | `""`           | 第三方 HTTP 代理的 URL。                                     |
| `CLOUD_PROVIDER_URL`      | `""`           | 部署在其他云服务商的兼容代理 URL。                           |
| `DOH_SERVER_HOSTNAME`     | `"dns.google"` | DoH 服务器的主机名。                                         |
| ...                       | ...            | (更多 DNS 相关配置请参考 `single.js` 代码)                   |

## 使用方法



### AIGateway (AI 代理优化版)

#### 1. 使用 URL 预设 (推荐)
代码内置了常用 AI 服务的别名，这是最简单方便的使用方式。
**URL 格式:** `https://<你的Worker地址>/<认证令牌>/<预设别名>/<API路径>`

**示例:**
- **OpenAI:** `https://your-worker.workers.dev/your-token/openai`
- **Gemini:** `https://your-worker.workers.dev/your-token/gemini`

#### 2. 使用完整 URL
**URL 格式:** `https://<你的Worker地址>/<认证令牌>/<完整的目标URL>`

### single.js (通用版)

通过构造特定的 URL 路径来使用代理功能。
**URL 基本格式:** `https://<你的Worker地址>/<认证令牌>/<完整的目标URL>`

#### 1. HTTP/HTTPS 代理
**URL 格式:** `https://<YOUR_WORKER_URL>/<AUTH_TOKEN>/被代理的URL（包含https://或http://协议头）`

#### 2. WebSocket (WSS) 代理
**URL 格式:** `wss://<YOUR_WORKER_URL>/<AUTH_TOKEN>/ws/<TARGET_WS_SERVER>`

#### 3. DoH/DOT
**URL 格式:** `https://<YOUR_WORKER_URL>/<AUTH_TOKEN>/dns/doh或dot` (详见代码)


## 兼容性测试
- **Google Gemini**: ✅
- **OpenAI**: ✅ 
- **Anthropic Claude**: ✅ 
- **NewAPI / One API 等聚合平台**: ✅
- **GPT-LOAD**：✅
- **Gemini-balance：✅**

## 开发指南

### 项目结构
(此结构为完整开发版，单文件版本已将所有代码合并)
```
├── README.md               # 项目说明文档
├── aigateway%.js            # AI代理优化版，单文件部署
├── single.js               # 通用版，单文件部署
└── src/                    # 源代码目录
    ...
```

### 第三方HTTP代理
(适用于 `single.js` 的 `thirdparty` 策略)

需要在第三方代理中实现：接收来自 Cloudflare Worker 的特殊 HTTP 请求，解析 URL中的 `target` 参数，然后向目标服务器发起新请求，并将响应原封不动地返回。

**关键点:**
1.  **获取 `target` 参数**: `[您的代理URL]?target=[原始目标URL]`
2.  **移除请求头**: 转发时移除 `Host`, `cf-*`, `x-forwarded-*` 等头。
3.  **返回响应**: 将目标服务器的响应（状态码、头、体）完整传回，移除 `Transfer-Encoding` 头。

### 其他云平台兼容代理
(适用于 `single.js` 的 `cloudprovider` 策略)

**核心工作原理**与第三方HTTP代理一致，但通常部署在 Vercel, Zeabur 等 Serverless 平台。可参考 `single.js` 内的示例代码进行开发。

您需要创建一个 Serverless Function，它会暴露一个公开的 URL (例如 `api/proxy.js`)。

当 Cloudflare Worker 调用它时，会在 URL 中附加一个关键参数 **`target`**，格式为： `[您的函数URL]?target=[原始目标URL]`

您的函数代码需要获取 `target` URL、请求方法、头和体。最核心的部分是使用 `fetch` API 将请求转发到 `target`。**关键在于处理请求头**：

- **必须移除 `Host` 头**，因为 `fetch` 会自动生成正确的 `Host`。
- 建议移除 `cf-*` 和 `x-forwarded-*` 系列的头，以隐藏代理链。

最后，将从目标服务器获取的 `Response` 对象**直接返回**即可，Serverless 平台会自动高效地处理流式响应。

示例代码，请勿直接使用，建议混淆：

```
export default async function handler(request) {
  // 1. 获取 target URL
  const { searchParams } = new URL(request.url);
  const targetUrl = searchParams.get('target');

  if (!targetUrl) {
    return new Response('Error: Missing "target" URL parameter.', { status: 400 });
  }

  // 安全检查，确保 target 是有效的 http/https URL
  try {
    const url = new URL(targetUrl);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
        throw new Error('Invalid protocol.');
    }
  } catch (err) {
    return new Response('Error: Invalid "target" URL parameter.', { status: 400 });
  }

  // 2. 构造转发请求的头部
  const forwardHeaders = new Headers(request.headers);
  forwardHeaders.delete('host'); // fetch 会自动设置

  for (const [key, value] of request.headers.entries()) {
      if (key.toLowerCase().startsWith('cf-') || key.toLowerCase().startsWith('x-forwarded-')) {
          forwardHeaders.delete(key);
      }
  }

  try {
    // 3. 使用 fetch 转发请求
    const response = await fetch(targetUrl, {
      method: request.method,
      headers: forwardHeaders,
      body: request.body,
      redirect: 'manual', // 必须设置，防止 fetch 自动处理重定向
    });

    // 4. 直接返回从目标服务器获取的响应
    return response;

  } catch (error) {
    return new Response(`Error connecting to target: ${error.message}`, { status: 502 });
  }
}

// 对于 Vercel Edge Runtime, 需额外配置
export const config = {
  runtime: 'edge',
};
```

## 变更记录 (Changelog)

**请访问** https://github.com/XyzenSun/SpectreProxy/blob/main/ChangeLogs.md

## License

This project is licensed under the MIT License.