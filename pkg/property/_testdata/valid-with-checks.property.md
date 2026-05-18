---
kind: property
id: email
data_type: string
description: An RFC 5322 email address.
checks:
  required: true
  max_length: 320
  pattern: "^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$"
  trim: true
  lowercase: true
---

# Property: email

## Description

A normalised email address.

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->

---
*This document follows the https://specscore.md/property-specification*
