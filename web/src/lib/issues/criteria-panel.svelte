<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Plus from "@lucide/svelte/icons/plus";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import PropertyPicker, { type PickerOption } from "$lib/issues/property-picker.svelte";
	import {
		dropEvidence,
		evidenceKinds,
		evidenceLabels,
		fileEvidence,
		needsAddress,
		provenLine,
		readCriteria,
		type AcceptanceCriterion,
		type CriteriaPanel,
		type CriterionEvidence,
		type EvidenceKind,
	} from "$lib/issues/criteria";

	let {
		workspaceId,
		issueId,
		issueVersion,
		canEdit = false,
		when,
	}: {
		workspaceId: string;
		issueId: string;
		issueVersion: number;
		canEdit?: boolean;
		when: (instant: string) => string;
	} = $props();

	const id = $props.id();

	let panel = $state.raw<CriteriaPanel>({ kind: "loading" });
	let failure = $state("");
	let working = $state(false);
	let filing = $state("");
	let kind = $state<EvidenceKind>("test");
	let label = $state("");
	let address = $state("");

	const criteria = $derived(panel.kind === "ready" ? panel.criteria : []);
	const counted = $derived(provenLine(criteria));

	const kindOptions = $derived<PickerOption[]>(
		evidenceKinds.map((choice) => ({
			value: choice.kind,
			label: choice.label,
			checked: choice.kind === kind,
		}))
	);

	const chosenKind = $derived(evidenceKinds.find((choice) => choice.kind === kind));

	$effect(() => {
		issueVersion;

		void refresh();
	});

	async function refresh() {
		const held = await readCriteria(workspaceId, issueId);

		if (!held) {
			panel = { kind: "unavailable" };

			return;
		}

		panel = held.length === 0 ? { kind: "empty" } : { kind: "ready", criteria: held };
	}

	function start(criterion: AcceptanceCriterion) {
		filing = criterion.id;
		kind = "test";
		label = "";
		address = "";
		failure = "";
	}

	async function file(criterion: AcceptanceCriterion) {
		if (label.trim() === "") return;

		if (needsAddress(kind) && address.trim() === "") {
			failure = "Say where this can be read, so somebody else can go and look.";

			return;
		}

		working = true;
		failure = "";

		const filed = await fileEvidence(workspaceId, issueId, {
			criterionId: criterion.id,
			kind,
			label: label.trim(),
			url: address.trim() || undefined,
		});

		working = false;

		if (!filed) {
			failure = "That could not be filed. Try again in a moment.";

			return;
		}

		filing = "";
		await refresh();
	}

	async function forget(evidence: CriterionEvidence) {
		working = true;
		failure = "";

		if (!(await dropEvidence(workspaceId, issueId, evidence.id))) {
			failure = "That could not be taken back.";
		}

		working = false;
		await refresh();
	}
</script>

{#if panel.kind !== "empty"}
	<section class="flex flex-col gap-1.5">
		<div class="flex items-center gap-2.5">
			<h2 class="min-w-0 flex-1">
				<span
					class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase"
				>
					Acceptance criteria
				</span>
			</h2>
			{#if counted}
				<span class="font-mono text-2xs text-muted-foreground">{counted}</span>
			{/if}
		</div>

		{#if failure}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{failure}</Alert.Description>
			</Alert.Root>
		{/if}

		{#if panel.kind === "loading"}
			<div class="h-16 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
		{:else if panel.kind === "unavailable"}
			<p class="text-md text-muted-foreground">
				We could not read what proves this issue's criteria.
			</p>
		{:else}
			<ul class="flex flex-col gap-1.5">
				{#each criteria as criterion (criterion.id)}
					<li class="flex flex-col gap-1.5 rounded-md border border-line-subtle p-2.5">
						<div class="flex items-start gap-2">
							<span class="min-w-0 flex-1 text-md text-ink-900">{criterion.text}</span>
							{#if criterion.proven}
								<span
									class="inline-flex shrink-0 items-center gap-1 font-mono text-2xs text-muted-foreground"
								>
									<Check class="size-3" aria-hidden="true" />
									Proven
								</span>
							{/if}
						</div>

						{#if criterion.evidence.length > 0}
							<ul class="flex flex-col gap-1">
								{#each criterion.evidence as evidence (evidence.id)}
									<li class="flex items-center gap-2">
										<span class="font-mono text-2xs text-muted-foreground">
											{evidenceLabels[evidence.kind]}
										</span>
										{#if evidence.url}
											<a
												href={evidence.url}
												target="_blank"
												rel="noreferrer"
												class="inline-flex min-w-0 items-center gap-1 truncate text-sm text-link underline-offset-2 hover:underline"
											>
												{evidence.label}
												<ExternalLink class="size-3 shrink-0" aria-hidden="true" />
											</a>
										{:else}
											<span class="min-w-0 truncate text-sm text-ink-900">{evidence.label}</span>
										{/if}
										<span class="text-2xs text-muted-foreground">
											{evidence.recordedByName ?? "Somebody"} ·
											<time datetime={evidence.recordedAt}>{when(evidence.recordedAt)}</time>
										</span>
										{#if evidence.stale}
											<span
												class="shrink-0 rounded-sm border border-warning/40 px-1 py-0.5 font-mono text-2xs text-warning"
												title="This was filed against “{evidence.criterionText}”"
											>
												Answers older wording
											</span>
										{/if}
										<span class="flex-1"></span>
										{#if canEdit}
											<Button
												variant="ghost"
												size="icon-xs"
												disabled={working}
												aria-label="Take back {evidence.label}"
												onclick={() => forget(evidence)}
											>
												<X aria-hidden="true" />
											</Button>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}

						{#if canEdit && filing === criterion.id}
							<div class="flex flex-col gap-2 rounded-md bg-paper-2 p-2">
								<PropertyPicker
									options={kindOptions}
									placeholder="What kind of proof…"
									onpick={(chosen) => (kind = chosen as EvidenceKind)}
									class="w-51.5"
								>
									{#snippet trigger(props)}
										<Button {...props} variant="secondary" size="sm" disabled={working}>
											{chosenKind?.label ?? "A test"}
										</Button>
									{/snippet}
								</PropertyPicker>

								<div class="flex flex-col gap-1">
									<Label for="{id}-label">What it is</Label>
									<Input
										id="{id}-label"
										bind:value={label}
										placeholder={chosenKind?.kind === "person"
											? "Rae opened the export and the file was valid"
											: "TestExportWritesAFile"}
										disabled={working}
									/>
								</div>

								{#if needsAddress(kind)}
									<div class="flex flex-col gap-1">
										<Label for="{id}-url">{chosenKind?.hint}</Label>
										<Input
											id="{id}-url"
											bind:value={address}
											placeholder="https://"
											disabled={working}
										/>
									</div>
								{/if}

								<div class="flex flex-wrap items-center gap-2">
									<Button
										size="sm"
										disabled={working || label.trim() === ""}
										onclick={() => file(criterion)}
									>
										{working ? "Filing" : "File it"}
									</Button>
									<Button
										variant="ghost"
										size="sm"
										disabled={working}
										onclick={() => (filing = "")}
									>
										Cancel
									</Button>
								</div>
							</div>
						{:else if canEdit}
							<div>
								<Button
									variant="ghost"
									size="sm"
									disabled={working}
									onclick={() => start(criterion)}
								>
									<Plus aria-hidden="true" />
									Add evidence
								</Button>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}
