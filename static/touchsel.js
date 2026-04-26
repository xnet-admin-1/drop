/**
 * Terminal Touch Selection - adapted from Acode Foundation
 * https://github.com/Acode-Foundation/Acode
 * License: MIT
 */
class TerminalTouchSelection {
	constructor(terminal, container, callbacks) {
		this.terminal = terminal;
		this.container = container;
		this.callbacks = callbacks || {};

		this.tapHoldDuration = 600;
		this.moveThreshold = 8;
		this.handleSize = 24;
		this.fingerOffset = 40;

		this.isSelecting = false;
		this.isHandleDragging = false;
		this.selectionStart = null;
		this.selectionEnd = null;
		this.currentSelection = null;

		this.touchStartTime = 0;
		this.touchStartPos = { x: 0, y: 0 };
		this.initialTouchPos = { x: 0, y: 0 };
		this.tapHoldTimeout = null;
		this.dragHandle = null;
		this.isSelectionTouchActive = false;
		this.pendingSelectionClearTouch = null;

		this.selectionOverlay = null;
		this.startHandle = null;
		this.endHandle = null;
		this.contextMenu = null;

		this.cellDimensions = { width: 0, height: 0 };
		this.boundHandlers = {};

		this.wasFocusedBeforeSelection = false;
		this.contextMenuShouldStayVisible = false;
		this.selectionProtected = false;
		this.protectionTimeout = null;

		this.scrollElement = null;
		this.isTerminalScrolling = false;
		this.scrollEndTimeout = null;
		this.scrollEndDelay = 100;

		this.init();
	}

	init() {
		this.createSelectionOverlay();
		this.createHandles();
		this.createContextMenu();
		this.attachEventListeners();
		this.updateCellDimensions();
	}

	createSelectionOverlay() {
		this.selectionOverlay = document.createElement("div");
		this.selectionOverlay.className = "terminal-selection-overlay";
		this.container.appendChild(this.selectionOverlay);
	}

	createHandles() {
		this.startHandle = this.createHandle("start");
		this.endHandle = this.createHandle("end");
		this.selectionOverlay.appendChild(this.startHandle);
		this.selectionOverlay.appendChild(this.endHandle);
	}

	createHandle(type) {
		const handle = document.createElement("div");
		handle.className = "terminal-selection-handle terminal-selection-handle-" + type;
		handle.dataset.handleType = type;
		return handle;
	}

	createContextMenu() {
		this.contextMenu = document.createElement("div");
		this.contextMenu.className = "terminal-context-menu";

		const items = [
			{ label: "Copy", action: () => this.copySelection() },
			{ label: "Paste", action: () => this.pasteFromClipboard() },
			{ label: "All", action: () => this.selectAllText() },
		];

		items.forEach(item => {
			const button = document.createElement("button");
			button.textContent = item.label;
			let acted = false;
			button.addEventListener("touchstart", e => { e.preventDefault(); e.stopPropagation(); acted = false; });
			button.addEventListener("touchend", e => { e.preventDefault(); e.stopPropagation(); if (!acted) { acted = true; item.action(); } });
			button.addEventListener("click", e => { e.preventDefault(); e.stopPropagation(); });
			this.contextMenu.appendChild(button);
		});

		this.selectionOverlay.appendChild(this.contextMenu);
	}

	attachEventListeners() {
		this.boundHandlers.terminalTouchStart = this.onTerminalTouchStart.bind(this);
		this.boundHandlers.terminalTouchMove = this.onTerminalTouchMove.bind(this);
		this.boundHandlers.terminalTouchEnd = this.onTerminalTouchEnd.bind(this);

		this.terminal.element.addEventListener("touchstart", this.boundHandlers.terminalTouchStart, { passive: false });
		this.terminal.element.addEventListener("touchmove", this.boundHandlers.terminalTouchMove, { passive: false });
		this.terminal.element.addEventListener("touchend", this.boundHandlers.terminalTouchEnd, { passive: false });
		this.terminal.element.addEventListener("contextmenu", e => e.preventDefault());

		this.boundHandlers.handleTouchStart = this.onHandleTouchStart.bind(this);
		this.boundHandlers.handleTouchMove = this.onHandleTouchMove.bind(this);
		this.boundHandlers.handleTouchEnd = this.onHandleTouchEnd.bind(this);

		[this.startHandle, this.endHandle].forEach(h => {
			h.addEventListener("touchstart", this.boundHandlers.handleTouchStart, { passive: false });
			h.addEventListener("touchmove", this.boundHandlers.handleTouchMove, { passive: false });
			h.addEventListener("touchend", this.boundHandlers.handleTouchEnd, { passive: false });
		});

		this.boundHandlers.selectionChange = this.onSelectionChange.bind(this);
		this.terminal.onSelectionChange(this.boundHandlers.selectionChange);

		this.boundHandlers.orientationChange = this.onOrientationChange.bind(this);
		window.addEventListener("orientationchange", this.boundHandlers.orientationChange);
		window.addEventListener("resize", this.boundHandlers.orientationChange);

		this.boundHandlers.terminalScroll = this.onTerminalScroll.bind(this);
		this.scrollElement = this.terminal.element.querySelector(".xterm-viewport") || this.terminal.element;
		this.scrollElement.addEventListener("scroll", this.boundHandlers.terminalScroll, { passive: true });

		this.boundHandlers.terminalResize = this.onTerminalResize.bind(this);
		this.terminal.onResize(this.boundHandlers.terminalResize);
	}

	onTerminalTouchStart(event) {
		if (event.touches.length !== 1) return;
		const touch = event.touches[0];
		this.touchStartTime = Date.now();
		this.touchStartPos = { x: touch.clientX, y: touch.clientY };
		this.initialTouchPos = { x: touch.clientX, y: touch.clientY };

		if (this.isSelecting) {
			this.isSelectionTouchActive = false;
			this.pendingSelectionClearTouch = { x: touch.clientX, y: touch.clientY, moved: false };
			this.hideContextMenu(true);
			return;
		}

		if (this.isEdgeGesture(touch)) return;

		if (this.tapHoldTimeout) clearTimeout(this.tapHoldTimeout);
		this.pendingSelectionClearTouch = null;
		this.isSelectionTouchActive = false;

		this.tapHoldTimeout = setTimeout(() => {
			if (!this.isSelecting) this.startSelection(touch);
		}, this.tapHoldDuration);
	}

	onTerminalTouchMove(event) {
		if (event.touches.length !== 1) return;
		const touch = event.touches[0];
		const deltaX = Math.abs(touch.clientX - this.touchStartPos.x);
		const deltaY = Math.abs(touch.clientY - this.touchStartPos.y);
		const horizontalDelta = touch.clientX - this.touchStartPos.x;
		const clearTouch = this.pendingSelectionClearTouch;

		if (clearTouch) {
			if (Math.abs(touch.clientX - clearTouch.x) > this.moveThreshold ||
				Math.abs(touch.clientY - clearTouch.y) > this.moveThreshold) {
				clearTouch.moved = true;
			}
		}

		if (this.isEdgeGesture(this.initialTouchPos) && Math.abs(horizontalDelta) > deltaY && deltaX > this.moveThreshold) {
			if (this.tapHoldTimeout) { clearTimeout(this.tapHoldTimeout); this.tapHoldTimeout = null; }
			return;
		}

		if (deltaX > this.moveThreshold || deltaY > this.moveThreshold) {
			if (this.tapHoldTimeout) { clearTimeout(this.tapHoldTimeout); this.tapHoldTimeout = null; }
			if (this.isSelecting && !this.isHandleDragging && this.isSelectionTouchActive) {
				event.preventDefault();
				this.extendSelection(touch);
			}
		}
	}

	onTerminalTouchEnd(event) {
		const hadTimer = !!this.tapHoldTimeout;
		if (this.tapHoldTimeout) { clearTimeout(this.tapHoldTimeout); this.tapHoldTimeout = null; }

		const shouldClear = this.isSelecting && !this.isHandleDragging &&
			this.pendingSelectionClearTouch && !this.pendingSelectionClearTouch.moved &&
			!this.isTerminalScrolling && !this.selectionProtected;

		this.pendingSelectionClearTouch = null;
		this.isSelectionTouchActive = false;

		if (shouldClear) { this.forceClearSelection(); this.terminal.focus(); return; }

		if (this.isSelecting && !this.isHandleDragging) {
			if (this.isTerminalScrolling) return;
			this.finalizeSelection();
		} else if (!this.isSelecting && hadTimer) {
			this.terminal.focus();
		}
	}

	onHandleTouchStart(event) {
		event.preventDefault();
		event.stopPropagation();
		if (event.touches.length !== 1) return;

		let handleType = event.target.dataset.handleType;
		if (!handleType) {
			if (event.target === this.startHandle || this.startHandle.contains(event.target)) handleType = "start";
			else if (event.target === this.endHandle || this.endHandle.contains(event.target)) handleType = "end";
		}
		if (!handleType) return;

		this.isHandleDragging = true;
		this.dragHandle = handleType;
		this.isSelectionTouchActive = false;
		this.pendingSelectionClearTouch = null;

		const touch = event.touches[0];
		this.initialTouchPos = { x: touch.clientX, y: touch.clientY };

		const targetHandle = handleType === "start" ? this.startHandle : this.endHandle;
		targetHandle.style.cursor = "grabbing";
		if (!targetHandle.style.transform.includes("scale")) {
			targetHandle.style.transform += " scale(1.2)";
		}
	}

	onHandleTouchMove(event) {
		if (!this.isHandleDragging || event.touches.length !== 1) return;
		event.preventDefault();
		event.stopPropagation();

		const touch = event.touches[0];
		const deltaX = Math.abs(touch.clientX - this.initialTouchPos.x);
		const deltaY = Math.abs(touch.clientY - this.initialTouchPos.y);
		if (deltaX < this.moveThreshold && deltaY < this.moveThreshold) return;

		const adjustedTouch = { clientX: touch.clientX, clientY: touch.clientY - this.fingerOffset };
		const coords = this.touchToTerminalCoords(adjustedTouch);
		if (!coords) return;

		if (this.dragHandle === "start") {
			this.selectionStart = coords;
			if (this.selectionEnd && (coords.row > this.selectionEnd.row ||
				(coords.row === this.selectionEnd.row && coords.col > this.selectionEnd.col))) {
				const temp = this.selectionStart;
				this.selectionStart = this.selectionEnd;
				this.selectionEnd = temp;
				this.dragHandle = "end";
			}
		} else {
			this.selectionEnd = coords;
			if (this.selectionStart && (coords.row < this.selectionStart.row ||
				(coords.row === this.selectionStart.row && coords.col < this.selectionStart.col))) {
				const temp = this.selectionEnd;
				this.selectionEnd = this.selectionStart;
				this.selectionStart = temp;
				this.dragHandle = "start";
			}
		}
		this.updateSelection();
	}

	onHandleTouchEnd(event) {
		if (!this.isHandleDragging) return;
		event.preventDefault();
		event.stopPropagation();

		this.isHandleDragging = false;
		this.dragHandle = null;

		[this.startHandle, this.endHandle].forEach(handle => {
			handle.style.cursor = "grab";
			handle.style.transform = handle.style.transform.replace(/\s*scale\([^)]*\)/g, "").trim();
		});

		this.finalizeSelection();
	}

	onSelectionChange() {
		if (!this.isSelecting) return;
		const selection = this.terminal.getSelection();
		if (selection && selection.length > 0) {
			this.currentSelection = selection;
			this.updateHandlePositions();
		}
	}

	onOrientationChange() {
		setTimeout(() => {
			this.updateCellDimensions();
			if (this.isSelecting) this.updateHandlePositions();
		}, 100);
	}

	onTerminalScroll() {
		if (!this.isSelecting || this.isHandleDragging) return;
		this.isTerminalScrolling = true;
		this.hideHandles();
		this.hideContextMenu(true);

		if (this.scrollEndTimeout) clearTimeout(this.scrollEndTimeout);
		this.scrollEndTimeout = setTimeout(() => {
			this.scrollEndTimeout = null;
			this.isTerminalScrolling = false;
			if (!this.isSelecting || this.isHandleDragging) return;
			this.updateHandlePositions();
			if (this.contextMenuShouldStayVisible) this.showContextMenu();
		}, this.scrollEndDelay);
	}

	onTerminalResize(size) {
		setTimeout(() => {
			this.updateCellDimensions();
			if (!this.isSelecting) return;
			if (this.selectionProtected) { this.updateHandlePositions(); return; }
			if (this.selectionStart && this.selectionEnd &&
				(this.selectionStart.row >= size.rows || this.selectionEnd.row >= size.rows)) {
				this.clearSelection();
			} else if (this.isSelecting) {
				this.updateHandlePositions();
				this.hideContextMenu(true);
				setTimeout(() => { if (this.isSelecting) this.showContextMenu(); }, 100);
			}
		}, 50);
	}

	startSelection(touch) {
		const coords = this.touchToTerminalCoords(touch);
		if (!coords) return;

		this.wasFocusedBeforeSelection = this.isTerminalFocused();
		this.selectionProtected = true;
		if (this.protectionTimeout) clearTimeout(this.protectionTimeout);
		this.protectionTimeout = setTimeout(() => { this.selectionProtected = false; }, 1000);

		this.isSelecting = true;
		this.isSelectionTouchActive = true;
		this.pendingSelectionClearTouch = null;

		this.selectionStart = coords;
		this.selectionEnd = { ...coords };

		this.terminal.clearSelection();
		this.updateSelection();
		this.currentSelection = this.terminal.getSelection();
		this.showHandles();
		this.showContextMenu();

		if (navigator.vibrate) navigator.vibrate(50);
	}

	extendSelection(touch) {
		const coords = this.touchToTerminalCoords(touch);
		if (!coords) return;
		this.selectionEnd = coords;
		this.updateSelection();
	}

	updateSelection() {
		if (!this.selectionStart || !this.selectionEnd) return;

		let startRow = this.selectionStart.row, startCol = this.selectionStart.col;
		let endRow = this.selectionEnd.row, endCol = this.selectionEnd.col;

		if (startRow > endRow || (startRow === endRow && startCol > endCol)) {
			[startRow, startCol, endRow, endCol] = [endRow, endCol, startRow, startCol];
		}

		const length = this.calculateSelectionLength(startRow, startCol, endRow, endCol);
		this.terminal.clearSelection();
		this.terminal.select(startCol, startRow, length);
		this.updateHandlePositions();

		if (this.contextMenuShouldStayVisible) this.showContextMenu();
	}

	calculateSelectionLength(startRow, startCol, endRow, endCol) {
		if (startRow === endRow) return endCol - startCol + 1;
		const cols = this.terminal.cols;
		return (cols - startCol) + (endRow - startRow - 1) * cols + (endCol + 1);
	}

	finalizeSelection() {
		if (this.currentSelection) this.showContextMenu();
	}

	showHandles() {
		this.startHandle.style.display = "block";
		this.endHandle.style.display = "block";
		this.updateHandlePositions();
	}

	hideHandles() {
		this.startHandle.style.display = "none";
		this.endHandle.style.display = "none";
	}

	getHandleBaseTransform(orientation) {
		return orientation === "start" ? "rotate(180deg) translateX(87%)" : "rotate(90deg) translateY(-13%)";
	}

	setHandleOrientation(handle, orientation) {
		if (!handle) return;
		const base = this.getHandleBaseTransform(orientation);
		const hasScale = /\bscale\(/.test(handle.style.transform || "");
		handle.dataset.orientation = orientation;
		handle.style.transform = hasScale ? base + " scale(1.2)" : base;
	}

	updateHandlePositions() {
		if (!this.selectionStart || !this.selectionEnd) return;

		let logicalStart, logicalEnd;
		if (this.selectionStart.row < this.selectionEnd.row ||
			(this.selectionStart.row === this.selectionEnd.row && this.selectionStart.col <= this.selectionEnd.col)) {
			logicalStart = this.selectionStart;
			logicalEnd = this.selectionEnd;
		} else {
			logicalStart = this.selectionEnd;
			logicalEnd = this.selectionStart;
		}

		const startPos = this.terminalCoordsToPixels(logicalStart);
		const endPos = this.terminalCoordsToPixels(logicalEnd);

		if (startPos) {
			this.startHandle.style.display = "block";
			this.startHandle.style.left = startPos.x + "px";
			this.startHandle.style.top = (startPos.y + this.cellDimensions.height + 4) + "px";
		} else {
			this.startHandle.style.display = "none";
		}

		if (endPos) {
			this.endHandle.style.display = "block";
			this.endHandle.style.left = (endPos.x + this.cellDimensions.width) + "px";
			this.endHandle.style.top = (endPos.y + this.cellDimensions.height + 4) + "px";
		} else {
			this.endHandle.style.display = "none";
		}

		this.setHandleOrientation(this.startHandle, "start");
		this.setHandleOrientation(this.endHandle, "end");
	}

	showContextMenu() {
		this.contextMenuShouldStayVisible = true;

		const startPos = this.selectionStart ? this.terminalCoordsToPixels(this.selectionStart) : null;
		const endPos = this.selectionEnd ? this.terminalCoordsToPixels(this.selectionEnd) : null;

		const menuWidth = this.contextMenu.offsetWidth || 200;
		const menuHeight = this.contextMenu.offsetHeight || 50;
		const containerRect = this.container.getBoundingClientRect();

		let menuX, menuY;

		if (startPos || endPos) {
			let centerX, baseY;
			if (startPos && endPos) { centerX = (startPos.x + endPos.x) / 2; baseY = Math.max(startPos.y, endPos.y); }
			else if (startPos) { centerX = startPos.x; baseY = startPos.y; }
			else { centerX = endPos.x; baseY = endPos.y; }

			menuX = centerX - menuWidth / 2;
			menuY = baseY + this.cellDimensions.height + 40;

			if (menuY > containerRect.height - menuHeight - 10) {
				const topY = startPos && endPos ? Math.min(startPos.y, endPos.y) : baseY;
				menuY = topY - menuHeight - 10;
			}
		} else {
			menuX = (containerRect.width - menuWidth) / 2;
			menuY = containerRect.height - menuHeight - 20;
		}

		menuX = Math.max(10, Math.min(menuX, containerRect.width - menuWidth - 10));
		menuY = Math.max(10, Math.min(menuY, containerRect.height - menuHeight - 10));

		this.contextMenu.style.left = menuX + "px";
		this.contextMenu.style.top = menuY + "px";
		this.contextMenu.style.display = "flex";
	}

	hideContextMenu(force) {
		if (this.contextMenu && (force || !this.contextMenuShouldStayVisible)) {
			this.contextMenu.style.display = "none";
		}
	}

	copySelection() {
		const text = (this.currentSelection || this.terminal.getSelection() || "").replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, '').replace(/\n\s*\n[\s\n]*/g, '\n').trim();
		if (text) navigator.clipboard.writeText(text).catch(() => {});
		this.forceClearSelection();
		if (this.callbacks.onCopy) this.callbacks.onCopy(text);
	}

	pasteFromClipboard() {
		navigator.clipboard.readText().then(text => {
			if (text) this.terminal.paste(text);
		}).catch(() => {});
		this.forceClearSelection();
	}

	selectAllText() {
		if (!this.terminal.selectAll) return;
		this.terminal.selectAll();
		this.currentSelection = this.terminal.getSelection();
		this.isSelecting = true;
		this.selectionStart = null;
		this.selectionEnd = null;
		this.selectionProtected = false;
		this.hideHandles();
		if (this.currentSelection) this.showContextMenu();
	}

	clearSelection() {
		if (this.selectionProtected) return;
		const shouldRestoreFocus = this.wasFocusedBeforeSelection && this.isSelecting;

		this.isSelecting = false;
		this.isHandleDragging = false;
		this.selectionStart = null;
		this.selectionEnd = null;
		this.currentSelection = null;
		this.dragHandle = null;
		this.pendingSelectionClearTouch = null;
		this.isSelectionTouchActive = false;
		this.isTerminalScrolling = false;

		this.terminal.clearSelection();
		this.hideHandles();
		this.contextMenu.style.display = "none";
		this.contextMenuShouldStayVisible = false;

		if (this.tapHoldTimeout) { clearTimeout(this.tapHoldTimeout); this.tapHoldTimeout = null; }
		if (this.scrollEndTimeout) { clearTimeout(this.scrollEndTimeout); this.scrollEndTimeout = null; }
		if (this.protectionTimeout) { clearTimeout(this.protectionTimeout); this.protectionTimeout = null; }
		this.selectionProtected = false;

		if (shouldRestoreFocus && !this.isTerminalFocused()) {
			setTimeout(() => { if (!this.isSelecting) this.terminal.focus(); }, 150);
		}
		this.wasFocusedBeforeSelection = false;
	}

	forceClearSelection() {
		this.selectionProtected = false;
		this.clearSelection();
	}

	touchToTerminalCoords(touch) {
		const rect = this.terminal.element.getBoundingClientRect();
		const x = touch.clientX - rect.left;
		const y = touch.clientY - rect.top;
		if (x < 0 || y < 0 || x > rect.width || y > rect.height) return null;

		const col = Math.floor(x / this.cellDimensions.width);
		const row = Math.floor(y / this.cellDimensions.height) + this.terminal.buffer.active.viewportY;
		return {
			col: Math.max(0, Math.min(col, this.terminal.cols - 1)),
			row: Math.max(0, row),
		};
	}

	terminalCoordsToPixels(coords) {
		const rect = this.terminal.element.getBoundingClientRect();
		const containerRect = this.container.getBoundingClientRect();
		const vy = this.terminal.buffer.active.viewportY;

		const x = coords.col * this.cellDimensions.width + (rect.left - containerRect.left);
		const y = (coords.row - vy) * this.cellDimensions.height + (rect.top - containerRect.top);

		const isVisible = coords.row >= vy && coords.row < vy + this.terminal.rows;
		return isVisible ? { x, y } : null;
	}

	updateCellDimensions() {
		if (this.terminal._core && this.terminal._core._renderService) {
			const dimensions = this.terminal._core._renderService.dimensions;
			if (dimensions && dimensions.css && dimensions.css.cell) {
				this.cellDimensions = {
					width: dimensions.css.cell.width,
					height: dimensions.css.cell.height,
				};
			}
		}
	}

	isTerminalFocused() {
		try {
			return document.activeElement === this.terminal.element ||
				this.terminal.element.contains(document.activeElement) ||
				(this.terminal._core && this.terminal._core._hasFocus);
		} catch (e) { return false; }
	}

	getWordBoundsAt(coords) {
		try {
			const buffer = this.terminal.buffer.active;
			const line = buffer.getLine(coords.row);
			if (!line) return null;
			const lineText = line.translateToString(false);
			if (!lineText || coords.col >= lineText.length) return null;
			const char = lineText[coords.col];
			if (!/[a-zA-Z0-9_\-.\/:~]/.test(char)) return null;

			let startCol = coords.col;
			while (startCol > 0 && /[a-zA-Z0-9_\-.\/:~]/.test(lineText[startCol - 1])) startCol--;
			let endCol = coords.col;
			while (endCol < lineText.length - 1 && /[a-zA-Z0-9_\-.\/:~]/.test(lineText[endCol + 1])) endCol++;

			if (endCol > startCol) {
				return { start: { row: coords.row, col: startCol }, end: { row: coords.row, col: endCol } };
			}
			return null;
		} catch (e) { return null; }
	}

	isEdgeGesture(touch) {
		const threshold = 30;
		return touch.clientX <= threshold || touch.clientX >= window.innerWidth - threshold;
	}

	destroy() {
		this.forceClearSelection();
		this.terminal.element.removeEventListener("touchstart", this.boundHandlers.terminalTouchStart);
		this.terminal.element.removeEventListener("touchmove", this.boundHandlers.terminalTouchMove);
		this.terminal.element.removeEventListener("touchend", this.boundHandlers.terminalTouchEnd);
		[this.startHandle, this.endHandle].forEach(h => {
			h.removeEventListener("touchstart", this.boundHandlers.handleTouchStart);
			h.removeEventListener("touchmove", this.boundHandlers.handleTouchMove);
			h.removeEventListener("touchend", this.boundHandlers.handleTouchEnd);
		});
		if (this.scrollElement) this.scrollElement.removeEventListener("scroll", this.boundHandlers.terminalScroll);
		window.removeEventListener("orientationchange", this.boundHandlers.orientationChange);
		window.removeEventListener("resize", this.boundHandlers.orientationChange);
		if (this.selectionOverlay && this.selectionOverlay.parentNode) {
			this.selectionOverlay.parentNode.removeChild(this.selectionOverlay);
		}
	}
}
