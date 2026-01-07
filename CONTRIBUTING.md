# <a name="contributing">Contributing Overview</a>
Please do! Thanks for your help improving the project! :balloon:

All contributors are welcome. Please see the [newcomers welcome guide](https://meshery.io/community/newcomers) for how, where and why to contribute. This project is community-built and welcomes collaboration. Contributors are expected to adhere to our [Code of Conduct](.CODE_OF_CONDUCT.md).

Not sure where to start? First, see the [newcomers welcome guide](https://meshery.io/community/newcomers). Grab an open issue with the [help-wanted label](../../labels/help%20wanted) and jump in. Join the [Slack account](http://slack.meshery.io) and engage in conversation. Create a [new issue](/../../issues/new/choose) if needed.  All [pull requests](/../../pulls) should reference an open [issue](/../../issues). Include keywords in your pull request descriptions, as well as commit messages, to [automatically close issues in GitHub](https://help.github.com/en/github/managing-your-work-on-github/closing-issues-using-keywords).

**Sections**
- <a name="contributing">General Contribution Flow</a>
  - <a href="#commit-signing">Developer Certificate of Origin</a>

Relevant coding style guidelines are the Go Code Review Comments and the Formatting and style section of Peter Bourgon's Go: Best Practices for Production Environments.

# <a name="contributing">General Contribution Flow</a>

In order to contribute to Meshery, please follow the fork-and-pull request workflow described [here](./CONTRIBUTING-gitflow.md).

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
