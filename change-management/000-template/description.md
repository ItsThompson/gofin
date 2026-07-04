# Description — <NNN>_<kebab-title>

> Copy this template with `cp -r 000-template <NNN>_<kebab-title>` and answer every
> prompt below. Do not add, remove, or rename the `##`/`####` headings: the validator
> enforces this exact structure. Replace each `<...>` placeholder with your content.

## Change Event

#### What is the purpose of this activity or change?

<why this change is being made, in one or two sentences>

#### What will be required to execute this change?

<people, access, tools, prerequisite deploys, backups>

#### What is the expected end state of the system after this change?

<the observable state once the change is complete>

#### What assumptions, if any, are being made about the state of the system at the time of this change?

<preconditions you are relying on; how they are confirmed in preflight>

#### Rollout Date/Time(s) and Duration

<planned window and expected duration, or "on demand">

## Impact / Risk Assessment

#### Why is it necessary? What is the impact of not making this change?

<consequence of doing nothing>

#### Why does this activity or change need to be done under Change Management? Can it be safely automated?

<why this is a managed manual operation rather than an automated migration>

#### Are there any related, prerequisite changes upon which this CM hinges?

<upstream PRs, deploys, or other CM items this depends on>

#### Will this CM be in any way intrusive, and if so, how will you know? What teams, services or functionality will be impacted?

<blast radius; services, data, or users affected and how impact is detected>

#### How has this change been tested to verify it's safe for production?

<tests, dry-runs, or staging rehearsals proving safety>

## Worst Case Scenario

#### What could happen if everything goes wrong with this change?

<the most severe realistic failure mode>

#### How does this CM attempt to mitigate this risk?

<scoping, transactions, backups, and other guardrails that reduce the risk>

## Rollback Procedure

#### What conditions would indicate a need to rollback?

<the signals that trigger a rollback decision>

#### In the event of problems, what will you do to return your system to a known good state?

<the concrete rollback actions, in order>

#### If this is a software or infrastructure change, has the rollback procedure been verified in a development environment?

<how the rollback was rehearsed, or "N/A" with justification>
