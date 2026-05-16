import React from "react";
import { UIStateContext } from "../providers/UIStateProvider.js";

export function useUIState() {
  const value = React.useContext(UIStateContext);
  if (!value) {
    throw new Error("useUIState must be used within UIStateProvider");
  }
  return value;
}
