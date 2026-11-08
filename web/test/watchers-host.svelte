<script lang="ts">
	import Watchers from "../src/lib/issues/watchers.svelte";
	import * as Tooltip from "../src/lib/components/ui/tooltip/index.js";
	import type { PersonDetail } from "../src/lib/workspace/members.js";

	let { watchers }: { watchers: PersonDetail[] } = $props();
</script>

<Tooltip.Provider>
	<Watchers {watchers} />
</Tooltip.Provider>
