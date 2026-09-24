import { persistentStore } from "./persistent";

export const modelsMenuOpen = persistentStore<boolean>("models-menu-open", true);
export const openModelFolders = persistentStore<string[]>("models-folders-open", []);
// Sidebar width in px set by dragging the rail; null means the default SIDEBAR_WIDTH.
export const sidebarWidth = persistentStore<number | null>("sidebar-width", null);
