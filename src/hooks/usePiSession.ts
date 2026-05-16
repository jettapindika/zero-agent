import React from "react";
import { PiSessionContext } from "../providers/PiSessionProvider.js";

export function usePiSession() {
  const value = React.useContext(PiSessionContext);
  if (!value) {
    throw new Error("usePiSession must be used within PiSessionProvider");
  }
  return value;
}
