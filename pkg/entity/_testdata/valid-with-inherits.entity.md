---
kind: entity
id: valid-with-inherits
singular: ValidChild
plural: ValidChildren
description: A child entity that inherits from a sibling parent fixture.
inherits: ./valid-minimal.entity.md
properties:
  - name: child_field
    data_type: string
    description: Extra field appended by the child.
    checks:
      required: false
---

# Entity: ValidChild

## Description

A child entity that exercises the inherits keyword.

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->

---
*This document follows the https://specscore.md/entity-specification*
