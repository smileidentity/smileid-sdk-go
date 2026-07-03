# Security policy

## Reporting a vulnerability

If you believe you have found a security vulnerability in this SDK, report it privately rather than opening a public issue.

Email: [security@smileidentity.com](mailto:security@smileidentity.com)

Please include:

- a description of the issue and its potential impact
- steps to reproduce, or a proof-of-concept if you have one
- any relevant code samples or logs, with sensitive data redacted
- your contact details, so we can follow up

We aim to acknowledge reports within 3 business days and to give a substantive response within 10 business days. Please give us a reasonable opportunity to fix the issue before any public disclosure.

## Scope

This repository contains the source code and tests for Smile ID's server-side SDK for Go. These reports are in scope and welcome:

- vulnerabilities in the SDK's source code, for example insecure handling of credentials, unsafe deserialisation, or injection issues
- vulnerabilities in this SDK's dependencies
- issues affecting the integrity of this repository, for example supply-chain concerns in the CI workflows

## Out of scope

- vulnerabilities in the underlying Smile ID API endpoints — report these to the same address, but note they belong to a separate system
- vulnerabilities in third-party services this SDK integrates with — report these to the relevant vendor
- findings that need physical access, social engineering, or denial-of-service testing against production endpoints
