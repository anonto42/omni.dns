# DNS Protocol Reference

## Overview

DNS (Domain Name System) resolves domain names to IP addresses. It uses UDP (usually port 53) with a simple binary format.

```
Client                     DNS Server
  │                            │
  │──── Query (UDP :53) ──────→│
  │   What is google.com?      │
  │                            │
  │←─── Response (UDP :53) ────│
  │   google.com = 142.250.xx  │
  │                            │
```

## Packet Structure

All multi-byte integers are **big-endian** (network byte order).

```
+-----------------------------+
|      Header (12 bytes)      |
+-----------------------------+
|      Question Section       |  (variable)
+-----------------------------+
|      Answer Section         |  (variable, empty in queries)
+-----------------------------+
|      Authority Section      |  (optional)
+-----------------------------+
|      Additional Section     |  (optional)
+-----------------------------+
```

## Header (12 bytes)

```
 0  1  2  3  4  5  6  7  8  9  10 11
+--+--+--+--+--+--+--+--+--+--+--+--+
|            ID (16 bits)            |
+--+--+--+--+--+--+--+--+--+--+--+--+
|QR|  OPCODE  |AA|TC|RD|RA| Z| RCODE |
+--+--+--+--+--+--+--+--+--+--+--+--+
|          QDCOUNT (16 bits)         |
+--+--+--+--+--+--+--+--+--+--+--+--+
|          ANCOUNT (16 bits)         |
+--+--+--+--+--+--+--+--+--+--+--+--+
|          NSCOUNT (16 bits)         |
+--+--+--+--+--+--+--+--+--+--+--+--+
|          ARCOUNT (16 bits)         |
+--+--+--+--+--+--+--+--+--+--+--+--+
```

### Field meanings

| Field | Size | Meaning |
|-------|------|---------|
| ID | 16 bits | Random identifier, paired with response |
| QR | 1 bit | 0 = query, 1 = response |
| OPCODE | 4 bits | 0 = standard query |
| AA | 1 bit | Authoritative Answer |
| TC | 1 bit | Truncated (TCP fallback needed) |
| RD | 1 bit | Recursion Desired |
| RA | 1 bit | Recursion Available |
| RCODE | 4 bits | Response code: 0=OK, 1=FormatErr, 2=ServFail, 3=NXDOMAIN |

## Question Section

Each question has:

```
 0  1  2  3  4  5  6  7  8  9  10 11 12 13 14 15
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   QNAME (variable)              |
|  (sequence of length-prefixed labels)           |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   QTYPE (16 bits)               |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   QCLASS (16 bits)              |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

### QNAME encoding

```
Example: "google.com" becomes:

  06 'g' 'o' 'o' 'g' 'l' 'e'  03 'c' 'o' 'm'  00
  └────── 6 bytes ──────┘     └── 3 bytes ──┘  ┘ null
```

Each label: 1 byte length + that many bytes of name.
End: single `0x00` byte.

### QTYPE values

| Value | Type | Meaning |
|-------|------|---------|
| 1 | A | IPv4 address |
| 28 | AAAA | IPv6 address |
| 5 | CNAME | Canonical name (alias) |
| 15 | MX | Mail exchange |
| 255 | ANY | All records |

### QCLASS

Almost always `1` (IN = Internet).

## Answer Section

Each answer record:

```
 0  1  2  3  4  5  6  7  8  9  10 11 12 13 14 15
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   NAME (variable or pointer)    |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   TYPE (16 bits)                |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   CLASS (16 bits)               |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   TTL (32 bits)                 |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   RDLENGTH (16 bits)            |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                   RDATA (variable)              |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

### Name compression

NAME often uses a pointer: `0xC0 0x0C` means "same name as in the question at byte 12" (saves space).

### RDATA for A record (TYPE=1)

4 bytes: the IP address, e.g. `0x8E 0xFA 0x01 0x01` = `142.250.1.1`

## Example: Parsing a Query

Raw bytes for a query asking for `google.com` type A:

```
hex: 12 34  01 00  00 01  00 00  00 00  00 00
     ──ID── ─FLAGS─ ─QDCNT─ ─ANCNT─ ─NSCNT─ ─ARCNT─
     06 67 6F 6F 67 6C 65 03 63 6F 6D 00
     └┬┘───────────────────────┘ └┬┘─────┘  │
     len=6  "google"            len=3 "com"  end
     00 01  00 01
     ─QTYPE─ ─QCLASS─
     A=1     IN=1
```

## Example: Building a Response

Set the same ID, set QR=1 in flags, copy question, append answer:

```
Header: 12 34  81 80  00 01  00 01  00 00  00 00
                            └ answer count = 1
Question: (copy from query)
Answer:   C0 0C  00 01  00 01  00 00 00 3C  00 04
         └ptr── └A─── └IN── └───TTL=60─── └len=4
          8E FA 01 01
         └───IP: 142.250.1.1
```

## Key Points

- **No string parsing** — the domain is encoded as length-prefixed labels
- **No structured data** — everything is raw bytes, you build it with bit shifts
- **UDP is unreliable** — DNS is designed to be simple (retry on timeout)
- **Packet max 512 bytes** (UDP), 4096+ with EDNS0 extension
- **ID must match** between query and response — this is how clients match replies
