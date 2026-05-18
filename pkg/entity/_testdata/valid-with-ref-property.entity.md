---
kind: entity
id: valid-with-ref-property
singular: ValidWithRef
plural: ValidWithRefs
description: Demonstrates a property item using ref instead of inline data_type.
properties:
  - name: id
    data_type: string
    description: Inline property used for sanity checking.
    checks:
      required: true
  - name: email
    ref: ./email.property.md
---

# Entity: ValidWithRef

## Description

Demonstrates both inline and ref-form property items.

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->

---
*This document follows the https://specscore.md/entity-specification*
