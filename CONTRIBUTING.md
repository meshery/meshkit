# <a name="contributing">Contributing Overview</a>
Please do! Thanks for your help improving the project! :balloon:

Functional pull requests must be validated and pushed through the **no-mistakes** quality gate (`no-mistakes axi run --intent "<goal>"` or `git push no-mistakes <branch>`), which writes the pipeline attestation required by CI. Documentation-only (`docs/**`) and CI-only (`.github/workflows/**`) pull requests are exempt. At most three pipeline runs per pull request; if the change is not green after three runs, stop and ask a maintainer.

All contributors are welcome. Please see the [newcomers welcome guide](https://meshery.io/community/newcomers) for how, where and why to contribute. This project is community-built and welcomes collaboration. Contributors are expected to adhere to our [Code of Conduct](.CODE_OF_CONDUCT.md).

Not sure where to start? First, see the [newcomers welcome guide](https://meshery.io/community/newcomers). Grab an open issue with the [help-wanted label](../../labels/help%20wanted) and jump in. Join the [Slack account](http://slack.meshery.io) and engage in conversation. Create a [new issue](/../../issues/new/choose) if needed.  All [pull requests](/../../pulls) should reference an open [issue](/../../issues). Include keywords in your pull request descriptions, as well as commit messages, to [automatically close issues in GitHub](https://help.github.com/en/github/managing-your-work-on-github/closing-issues-using-keywords).

**Sections**
- <a href="#contributing-flow">General Contribution Flow</a>
  - <a href="#no-mistakes-gate">Quality Gate: no-mistakes</a>
  - <a href="#commit-signing">Developer Certificate of Origin</a>

Relevant coding style guidelines are the Go Code Review Comments and the Formatting and style section of Peter Bourgon's Go: Best Practices for Production Environments.

# <a name="contributing-flow">General Contribution Flow</a>

In order to contribute to Meshery, please follow the fork-and-pull request workflow described [here](./CONTRIBUTING-gitflow.md).

## <a name="no-mistakes-gate">Quality Gate: no-mistakes</a>

A pull request that changes any path outside documentation and CI merges only with a green `no-mistakes` run on its current head.
- **Exempt paths**: Documentation is `docs/**` (MeshKit publishes no website from this repository) and CI is `.github/workflows/**`.
- **Required check**: The `PR must be raised via no-mistakes` CI gate verifies that `review`, `test`, and `document` phases are each `completed` in a head-bound attestation.

### Fork Initialization and Setup

When contributing from a personal fork, configure your local clone so that `origin` points to upstream `meshery/meshkit` and initialize `no-mistakes` with `--fork-url` pointing to your personal fork:

1. **Set `origin` to upstream and initialize `no-mistakes`**:
   ```sh
   # Using HTTPS:
   git remote set-url origin https://github.com/meshery/meshkit.git
   no-mistakes init --fork-url https://github.com/<your-username>/meshkit.git

   # Using SSH:
   git remote set-url origin git@github.com:meshery/meshkit.git
   no-mistakes init --fork-url git@github.com:<your-username>/meshkit.git
   ```

2. **Create a feature branch**:
   Always work on a dedicated feature branch rather than the repository's default branch:
   ```sh
   git checkout -b <branch-name>
   ```

3. **Sign off on every commit**:
   Make your changes and sign off each commit (`git commit -s`, see [DCO sign-off](#commit-signing) below).

4. **Validate and submit through `no-mistakes`**:
   Run the `no-mistakes` pipeline to validate your changes, execute tests, and open or update the pull request:
   ```sh
   no-mistakes axi run --intent "<what you set out to accomplish>"
   ```
   Or push directly to the configured `no-mistakes` remote:
   ```sh
   git push no-mistakes <branch-name>
   ```

5. **Pipeline iteration**:
   At most three pipeline runs per pull request. If the run pauses at a decision gate, inspect with `no-mistakes axi status` and respond with `no-mistakes axi respond`. For more details, see [AGENTS.md](./AGENTS.md) and [.agents/skills/no-mistakes/SKILL.md](./.agents/skills/no-mistakes/SKILL.md), or refer to [meshery/meshery's CONTRIBUTING.md](https://github.com/meshery/meshery/blob/master/CONTRIBUTING.md) and the [Meshery Contributing Guide](https://docs.meshery.io/project/contributing).

## <a name="commit-signing">Signing-off on Commits (Developer Certificate of Origin)</a>

To contribute to this project, you must agree to the Developer Certificate of
Origin (DCO) for each commit you make. The DCO is a simple statement that you,
as a contributor, have the legal right to make the contribution.

See the [DCO](https://developercertificate.org) file for the full text of what you must agree to
and how it works [here](https://github.com/probot/dco#how-it-works).
To signify that you agree to the DCO for contributions, you simply add a line to each of your
git commit messages:

```
Signed-off-by: Jane Smith <jane.smith@example.com>
```

In most cases, you can add this signoff to your commit automatically with the
`-s` or `--signoff` flag to `git commit`. You must use your real name and a reachable email
address (sorry, no pseudonyms or anonymous contributions). An example of signing off on a commit:
```
$ commit -s -m “my commit message w/signoff”
```

To ensure all your commits are signed, you may choose to add this alias to your global ```.gitconfig```:

*~/.gitconfig*
```
[alias]
  amend = commit -s --amend
  cm = commit -s -m
  commit = commit -s
```
Or you may configure your IDE, for example, Visual Studio Code to automatically sign-off commits for you:

<a href="https://user-images.githubusercontent.com/7570704/64490167-98906400-d25a-11e9-8b8a-5f465b854d49.png" ><img src="https://user-images.githubusercontent.com/7570704/64490167-98906400-d25a-11e9-8b8a-5f465b854d49.png" width="50%"><a>


#### Tests
Users can now test their code on their local machine against the CI checks implemented using `make run-tests`.

To test code changes on your local machine, run the following command:
```
make run-tests
```

#### Building Docker image
To build a Docker image of the project, please ensure you have `Docker` installed to be able to build the image. Now, run the following command to build the Docker image:
```sh
make docker
```

### UI Lint Rules

Meshery uses ES-Lint to maintain code quality & consistency in our UI Code.

# <a name="maintaining"> Reviews</a>
All contributors are invited to review pull requests. See this short video on [how to review a pull request](https://www.youtube.com/watch?v=isLfo7jfE6g&feature=youtu.be).

# New to Git?
Resources: https://lab.github.com and https://try.github.com/


# FAQs

### Instructions for making custom dictionary entries

Enter new key:value pairs into the `useDictionary()` map in https://github.com/meshery/meshkit/blob/master/utils/utils.go

- Key: string to match
- Value: string to display

The custom dictionary entries are case insensitive.

### License

This repository and site are available as open source under the terms of the [Apache 2.0 License](https://opensource.org/licenses/Apache-2.0).

### About Meshery

**Community First**
<p>The <a href="https://meshery.io">Meshery</a> community represents the largest collection of service mesh projects and their maintainers in the world.</p>

**Open Source First**
<p>Our projects establish industry standards and enable service developers, owners, and operators with repeatable patterns and best practices for managing all aspects of distributed services. Our shared commitment to the open source spirit push the Meshery community and its projects forward.</p>
