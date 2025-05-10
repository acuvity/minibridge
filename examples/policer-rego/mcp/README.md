# Rego Policy: Covert‑Instruction & Sensitive‑Data Guard

This folder contains an **Open Policy Agent (OPA) Rego v1** policy designed to safeguard a Model‑Context‑Protocol (MCP)‑compatible server from two common risks:

1. **Covert instructions** that attempt to hide or smuggle information through tool metadata or arguments.
2. **Accidental exposure of sensitive terms** (e.g., API tokens) in requests and responses.

It also provides **automatic redaction** of predefined keywords in tool call responses.

---

## 1 . High‑level Behavior

| Phase                              | What the policy checks / does                                                                                           | Outcome if triggered                      |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ----------------------------------------- |
| **tools/list**                     | ‑ Scans each tool description for _covert instructions_.<br>‑ Scans each description for _sensitive resource_ keywords. | `reasons` with a clear message.           |
| **tools/call**                     | ‑ Scans the **arguments** for covert instructions.<br>‑ Scans the arguments for sensitive resource keywords.            | `reasons` with a clear message.           |
| **tools/call (response mutation)** | ‑ Redacts text elements that match a _redaction pattern_ before the response leaves the policy.                         | Replaces matching text with `[REDACTED]`. |
| **allow**                          | Grants access **only** when `reasons` is empty.                                                                         | Request proceeds.                         |

---

## 2 . Input Shape (MCP v2025‑03‑26)

The policy follows the [MCP "calling‑tools" specification](https://modelcontextprotocol.io/specification/2025-03-26/server/tools#calling-tools). Key fields referenced in the policy are:

```rego
input.mcp.method             # "tools/list" or "tools/call"
input.mcp.params.arguments    # map → serialized into a string
input.mcp.result.tools        # array of {name, description}
input.mcp.result.content      # array of content items returned by a tool call
```

> **Tip**  If you are integrating with a different MCP version, adapt the field paths accordingly.

---

## 3 . Pattern Sets

| Variable              | Purpose                                           | Default value                            |
| --------------------- | ------------------------------------------------- | ---------------------------------------- |
| `_covert_patterns`    | Detect hidden or obfuscated instructions.         | `["(?i)hide this"]` _(case‑insensitive)_ |
| `_sensitive_patterns` | Detect direct mentions of sensitive resources.    | `["\\btoken\\b"]`                        |
| `_redaction_patterns` | Terms that must be removed from outgoing content. | `["\\bbluefin\\b"]`                      |

You can extend these arrays or load them dynamically from external data sources.

---

## 4 . Examples

### 4.1  Blocking a Covert Tool

```json
{
  "mcp": {
    "method": "tools/list",
    "result": {
      "tools": [
        {
          "name": "super_search",
          "description": "Quickly **hide this** query from logs."
        }
      ]
    }
  }
}
```

*Result* → `allow: false, reasons[0]="covert instruction in tool super_search: (?i)hide this"`

### 4.2  Redacting a Sensitive Term

```json
{
  "mcp": {
    "method": "tools/call",
    "result": {
      "content": [{ "type": "text", "text": "Here is the Bluefin manual." }]
    }
  }
}
```

*Result* → `allow: true`, The response text becomes `[REDACTED]`.

---

### 5. Testing Locally

```sh
opa test .
```
