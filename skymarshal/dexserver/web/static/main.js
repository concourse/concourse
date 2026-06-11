if (String(performance.getEntriesByType("navigation")[0].type) === "back_forward") {
  location.reload()
}
