export type PanelName = "sidebar" | "properties";

export type PanelBounds = {
	minimum: number;
	maximum: number;
	ordinary: number;
};

export type PanelSize = {
	width: number;
	collapsed: boolean;
};

export type PanelSizes = Record<PanelName, PanelSize>;

const bounds: Record<PanelName, PanelBounds> = {
	sidebar: { minimum: 176, maximum: 400, ordinary: 224 },
	properties: { minimum: 240, maximum: 480, ordinary: 300 },
};

const contentMinimum = 480;

export function panelBounds(name: PanelName): PanelBounds {
	return bounds[name];
}

export function panelCookie(name: PanelName): string {
	return `norn.panel.${name}`;
}

export function clampPanel(name: PanelName, width: number, room?: number): number {
	const { minimum, maximum } = bounds[name];
	const ceiling = room === undefined ? maximum : Math.min(maximum, room - contentMinimum);

	return Math.round(Math.min(Math.max(width, minimum), Math.max(ceiling, minimum)));
}

export function readPanel(name: PanelName, remembered: string | undefined): PanelSize {
	const ordinary = { width: bounds[name].ordinary, collapsed: false };

	if (!remembered) return ordinary;

	const [width, state] = remembered.split(":");
	const measured = Number(width);

	if (!Number.isFinite(measured)) return ordinary;

	return { width: clampPanel(name, measured), collapsed: state === "collapsed" };
}

export function writePanel(size: PanelSize): string {
	return `${size.width}:${size.collapsed ? "collapsed" : "open"}`;
}

export const ordinaryPanels: PanelSizes = {
	sidebar: { width: bounds.sidebar.ordinary, collapsed: false },
	properties: { width: bounds.properties.ordinary, collapsed: false },
};
