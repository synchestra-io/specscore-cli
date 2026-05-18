---
kind: entity
id: duplicate-property-name
singular: DupeProp
plural: DupeProps
properties:
  - name: email
    data_type: string
    checks:
      required: true
  - name: email
    data_type: string
    checks:
      required: false
---

# Entity: DupeProp

## Description

Two property items share the same name; lint must flag this.

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->

---
*This document follows the https://specscore.md/entity-specification*
