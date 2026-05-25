# Security Policy

## Supported Versions

Security fixes are provided for the latest released version and the current `main` branch.

## Reporting a Vulnerability

Please report suspected vulnerabilities privately using GitHub private vulnerability reporting if it is enabled for this repository:

https://github.com/mitchellrj/timeflip-go/security/advisories/new

If private vulnerability reporting is unavailable, open a minimal public issue that asks for a private contact path, but do not include exploit details, secrets, device identifiers, raw BLE captures containing passwords, or sensitive payloads in the public issue.

## What To Include

Include enough detail to reproduce and assess the issue:

- Affected version, commit, or tag.
- The vulnerable package, API, CLI command, or BLE workflow.
- Reproduction steps or a minimal proof of concept.
- Expected and actual behavior.
- Impact and any known mitigations.
- Whether the report includes raw BLE payloads or device-specific data.

## Handling Expectations

I will acknowledge valid reports as soon as practical, assess scope and impact, and coordinate a fix before public disclosure where appropriate. Public advisories and CVEs may be used for issues that materially affect downstream users.

## Scope

In scope:

- Vulnerabilities in this Go library, demo CLI, release workflows, and provenance configuration.
- Bugs that expose sensitive TimeFlip passwords, raw BLE payloads, or device identifiers unexpectedly.
- Supply-chain issues in build, release, or provenance artifacts.

Out of scope:

- Vulnerabilities in TimeFlip device firmware, hardware, mobile apps, or cloud services not maintained here.
- Reports requiring physical access without a software impact in this repository.
- Denial-of-service reports that only affect a local demo session without persistence or data exposure.
