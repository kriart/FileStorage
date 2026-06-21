function setupDialogs() {
  const clearAutoOpenParams = (dialog) => {
    if (!dialog || !dialog.hasAttribute("data-auto-open")) {
      return;
    }
    const url = new URL(window.location.href);
    let changed = false;
    ["linkFileId", "createdLink"].forEach((key) => {
      if (url.searchParams.has(key)) {
        url.searchParams.delete(key);
        changed = true;
      }
    });
    if (changed) {
      window.history.replaceState({}, "", `${url.pathname}${url.search}${url.hash}`);
    }
    dialog.removeAttribute("data-auto-open");
  };

  document.querySelectorAll("[data-dialog-target]").forEach((button) => {
    button.addEventListener("click", () => {
      const menu = button.closest(".file-menu");
      if (menu) {
        menu.removeAttribute("open");
      }

      const dialog = document.getElementById(button.dataset.dialogTarget);
      if (dialog && typeof dialog.showModal === "function") {
        dialog.showModal();
      }
    });
  });

  document.querySelectorAll("[data-dialog-close]").forEach((button) => {
    button.addEventListener("click", () => {
      const dialog = button.closest("dialog");
      if (dialog) {
        dialog.close();
        clearAutoOpenParams(dialog);
      }
    });
  });

  document.querySelectorAll("dialog").forEach((dialog) => {
    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) {
        dialog.close();
        clearAutoOpenParams(dialog);
      }
    });

    dialog.addEventListener("cancel", () => {
      clearAutoOpenParams(dialog);
    });

    dialog.addEventListener("close", () => {
      clearAutoOpenParams(dialog);
    });
  });

  document.querySelectorAll("dialog[data-auto-open]").forEach((dialog) => {
    if (typeof dialog.showModal === "function") {
      dialog.showModal();
      return;
    }
    dialog.setAttribute("open", "");
  });
}

function setupConfirmations() {
  document.querySelectorAll("form[data-confirm]").forEach((form) => {
    form.addEventListener("submit", (event) => {
      if (!window.confirm(form.dataset.confirm)) {
        event.preventDefault();
      }
    });
  });

  document.querySelectorAll("[data-auto-submit]").forEach((input) => {
    input.addEventListener("change", () => {
      if (!input.files || input.files.length === 0) {
        return;
      }
      const form = input.closest("form");
      if (!form) {
        return;
      }
      if (form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
        input.value = "";
        return;
      }
      form.submit();
    });
  });
}

function setupMenus() {
  document.addEventListener("click", (event) => {
    document.querySelectorAll(".file-menu[open], .language-menu[open]").forEach((menu) => {
      if (!menu.contains(event.target)) {
        menu.removeAttribute("open");
      }
    });
  });
}

function setupFolderLinks() {
  document.querySelectorAll("[data-folder-link]").forEach((node) => {
    node.value = `${window.location.origin}${node.dataset.folderLink}`;
  });
}

function setupCopyButtons() {
  document.querySelectorAll("[data-copy-value], [data-copy-from-folder-link]").forEach((button) => {
    button.addEventListener("click", async () => {
      const value = button.dataset.copyValue || `${window.location.origin}${button.dataset.copyFromFolderLink}`;
      const markCopied = () => {
        const copyLabel = button.dataset.copyLabel || button.textContent || "Копировать";
        const copiedLabel = button.dataset.copiedLabel || "Скопировано";
        button.textContent = copiedLabel;
        window.setTimeout(() => {
          button.textContent = copyLabel;
        }, 1400);
      };

      if (navigator.clipboard && window.isSecureContext) {
        try {
          await navigator.clipboard.writeText(value);
          markCopied();
          return;
        } catch (_) {
          // Fall back to a temporary field below.
        }
      }

      const area = document.createElement("textarea");
      area.value = value;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.left = "-9999px";
      document.body.append(area);
      area.select();
      const copied = document.execCommand("copy");
      area.remove();

      const row = button.closest(".copy-row");
      const input = row ? row.querySelector("input") : null;
      if (input) {
        input.value = value;
        input.select();
        input.setSelectionRange(0, input.value.length);
      }
      if (copied) {
        markCopied();
      }
    });
  });
}

function setupToasts() {
  document.querySelectorAll(".toast").forEach((toast) => {
    window.setTimeout(() => {
      toast.classList.add("is-hiding");
      window.setTimeout(() => {
        const stack = toast.closest(".toast-stack");
        toast.remove();
        if (stack && stack.children.length === 0) {
          stack.remove();
        }
      }, 220);
    }, 3600);
  });
}

function setupTypeEditors() {
  document.querySelectorAll("[data-type-editor]").forEach((editor) => {
    const valueField = editor.querySelector(".type-editor-value");
    const input = editor.querySelector(".type-editor-input");
    const addButton = editor.querySelector("[data-type-add]");
    const list = editor.querySelector("[data-type-list]");
    const form = editor.closest("form");
    const rules = [];

    const normalizeRule = (value) => {
      value = value.trim().toLowerCase().replace(/^\*/, "");
      if (!value) {
        return "";
      }
      if (value.includes("/")) {
        return value;
      }
      const dotIndex = value.lastIndexOf(".");
      if (dotIndex > 0 && dotIndex < value.length - 1) {
        return value.slice(dotIndex);
      }
      if (value.startsWith(".")) {
        return value;
      }
      return `.${value}`;
    };

    const sync = () => {
      valueField.value = rules.join("\n");
    };

    const render = () => {
      list.innerHTML = "";
      if (rules.length === 0) {
        const empty = document.createElement("span");
        empty.className = "type-empty";
        empty.textContent = editor.dataset.emptyLabel || "Любые файлы";
        list.append(empty);
        sync();
        return;
      }

      rules.forEach((rule) => {
        const chip = document.createElement("span");
        chip.className = "type-chip";

        const text = document.createElement("span");
        text.textContent = rule;
        chip.append(text);

        const remove = document.createElement("button");
        remove.type = "button";
        remove.setAttribute("aria-label", `Удалить ${rule}`);
        remove.textContent = "x";
        remove.addEventListener("click", () => {
          const index = rules.indexOf(rule);
          if (index >= 0) {
            rules.splice(index, 1);
            render();
          }
        });
        chip.append(remove);
        list.append(chip);
      });
      sync();
    };

    const addValues = (value) => {
      value.split(/[\s,;]+/).map(normalizeRule).filter(Boolean).forEach((rule) => {
        if (!rules.includes(rule)) {
          rules.push(rule);
        }
      });
      input.value = "";
      render();
    };

    addValues(valueField.value);

    addButton.addEventListener("click", () => {
      addValues(input.value);
      input.focus();
    });

    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") {
        event.preventDefault();
        addValues(input.value);
      }
    });

    if (form) {
      form.addEventListener("submit", () => {
        addValues(input.value);
      });
    }
  });
}

document.addEventListener("DOMContentLoaded", () => {
  setupFolderLinks();
  setupCopyButtons();
  setupDialogs();
  setupConfirmations();
  setupMenus();
  setupToasts();
  setupTypeEditors();
});
