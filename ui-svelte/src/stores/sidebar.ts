import { persistentStore } from "./persistent";

export const modelsMenuOpen = persistentStore<boolean>("models-menu-open", true);
export const openModelFolders = persistentStore<string[]>("models-folders-open", []);
