# Contributing to KAI Scheduler

Thank you for your interest in contributing to KAI Scheduler! This document provides guidelines and instructions to help you get started with contributing to our project.

Make sure to read our [Contributor License Agreement](CLA.md) and [Code of Conduct](code_of_conduct.md).

## Getting Started
### New Contributors
We're excited to help you make your first contribution! Whether you're interested in filing issues, developing features, fixing bugs, or improving documentation, we're here to support you through the process.

Browse issues labeled [good first issue] or [help wanted] on GitHub for an easy introduction.

### Developers
The main building blocks of KAI Scheduler are documented in the `docs/developer` folder. Here are the key components:
- [Action Framework](docs/developer/action-framework.md): Core scheduler logic
- [Plugin Framework](docs/developer/plugin-framework.md): Extensible plugin system
- [Pod Grouper](docs/developer/pod-grouper.md): Group scheduling functionality
- [Binder](docs/developer/binder.md): Binding logic

We recommend reading these documents to understand the architecture before making significant contributions.

## How to Contribute
### Reporting Issues
Open an issue with a clear description, steps to reproduce, and relevant environment details.
Use our issue templates for guidance.

### Improving Documentation
Help us keep the docs clear and useful by fixing typos, updating outdated information, or adding examples.

### Code & Documentation
- Fork & Clone: Start by forking the repository and cloning it locally.
- Create a Branch: Use a descriptive name (e.g., feature/add-cool-feature or bugfix/fix-issue123).
- Make Changes: Keep commits small and focused. For detailed build and test instructions, see [Building from Source](docs/developer/building-from-source.md).
- Submit a PR: Open a pull request, reference any related issues, and follow our commit message guidelines.

### Pull Request Checklist
Before introducing major changes, it's strongly recommended to open a PR outlining the proposed design.
Each PR should meet the following requirements:
- All tests pass. To run the full test suite locally, use: make build validate test
- Any affected code is covered by new or updated tests
- Relevant documentation has been added or updated

## Getting Help
If you have questions or need assistance:
- Open an issue on [GitHub](https://github.com/NVIDIA/KAI-Scheduler/issues).
- Join our community discussions (we are using [#batch-wg](https://cloud-native.slack.com/archives/C02Q5DFF3MM) Slack channel).

## License
By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

Thank you for your interest and happy coding!
