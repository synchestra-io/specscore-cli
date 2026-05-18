# Entity: Wrong Order

Some prose before the frontmatter — this is a violation of
[entity#req:frontmatter-required] which mandates the frontmatter as the
very first block. The parser must surface that the frontmatter is not in
the leading position so lint can flag it.

---
kind: entity
id: frontmatter-not-first-block
singular: WrongOrder
plural: WrongOrders
properties: []
---

## Description

The body continues here.

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->

---
*This document follows the https://specscore.md/entity-specification*
