/**!
 * Sortable
 * @author	RubaXa   <trash@rubaxa.org>
 * @license MIT
 */

(function (factory) {
	"use strict";

	if (typeof define === "function" && define.amd) {
		define(factory);
	}
	else if (typeof module != "undefined" && typeof module.exports != "undefined") {
		module.exports = factory();
	}
	else if (typeof Package !== "undefined") {
		Sortable = factory();  // export for Meteor.js
	}
	else {
		/* jshint sub:true */
		window["Sortable"] = factory();
	}
})(function () {
	"use strict";

	var dragEl,
		ghostEl,
		cloneEl,
		rootEl,
		nextEl,

		scrollEl,
		scrollParentEl,

		lastEl,
		lastCSS,

		oldIndex,
		newIndex,

		activeGroup,
		autoScroll = {},

		tapEvt,
		touchEvt,

		/** @const */
		RSPACE = /\s+/g,

		expando = 'Sortable' + (new Date).getTime(),

		win = window,
		document = win.document,
		parseInt = win.parseInt,

		supportDraggable = !!('draggable' in document.createElement('div')),

		_silent = false,

		_dispatchEvent = function (sortable, rootEl, name, targetEl, fromEl, startIndex, newIndex) {
			var evt = document.createEvent('Event'),
				options = (sortable || rootEl[expando]).options,
				onName = 'on' + name.charAt(0).toUpperCase() + name.substr(1);

			evt.initEvent(name, true, true);

			evt.item = targetEl || rootEl;
			evt.from = fromEl || rootEl;
			evt.clone = cloneEl;

			evt.oldIndex = startIndex;
			evt.newIndex = newIndex;

			if (options[onName]) {
				options[onName].call(sortable, evt);
			}

			rootEl.dispatchEvent(evt);
		},

		abs = Math.abs,
		slice = [].slice,

		touchDragOverListeners = [],

		_autoScroll = _throttle(function (/**Event*/evt, /**Object*/options, /**HTMLElement*/rootEl) {
			// Bug: https://bugzilla.mozilla.org/show_bug.cgi?id=505521
			if (rootEl && options.scroll) {
				var el,
					rect,
					sens = options.scrollSensitivity,
					speed = options.scrollSpeed,

					x = evt.clientX,
					y = evt.clientY,

					winWidth = window.innerWidth,
					winHeight = window.innerHeight,

					vx,
					vy
				;

				// Delect scrollEl
				if (scrollParentEl !== rootEl) {
					scrollEl = options.scroll;
					scrollParentEl = rootEl;

					if (scrollEl === true) {
						scrollEl = rootEl;

						do {
							if ((scrollEl.offsetWidth < scrollEl.scrollWidth) ||
								(scrollEl.offsetHeight < scrollEl.scrollHeight)
							) {
								break;
							}
							/* jshint boss:true */
						} while (scrollEl = scrollEl.parentNode);
					}
				}

				if (scrollEl) {
					el = scrollEl;
					rect = scrollEl.getBoundingClientRect();
					vx = (abs(rect.right - x) <= sens) - (abs(rect.left - x) <= sens);
					vy = (abs(rect.bottom - y) <= sens) - (abs(rect.top - y) <= sens);
				}


				if (!(vx || vy)) {
					vx = (winWidth - x <= sens) - (x <= sens);
					vy = (winHeight - y <= sens) - (y <= sens);

					/* jshint expr:true */
					(vx || vy) && (el = win);
				}


				if (autoScroll.vx !== vx || autoScroll.vy !== vy || autoScroll.el !== el) {
					autoScroll.el = el;
					autoScroll.vx = vx;
					autoScroll.vy = vy;

					clearInterval(autoScroll.pid);

					if (el) {
						autoScroll.pid = setInterval(function () {
							if (el === win) {
								win.scrollTo(win.pageXOffset + vx * speed, win.pageYOffset + vy * speed);
							} else {
								vy && (el.scrollTop += vy * speed);
								vx && (el.scrollLeft += vx * speed);
							}
						}, 24);
					}
				}
			}
		}, 30)
	;



	/**
	 * @class  Sortable
	 * @param  {HTMLElement}  el
	 * @param  {Object}       [options]
	 */
	function Sortable(el, options) {
		this.el = el; // root element
		this.options = options = _extend({}, options);


		// Export instance
		el[expando] = this;


		// Default options
		var defaults = {
			group: Math.random(),
			sort: true,
			disabled: false,
			store: null,
			handle: null,
			scroll: true,
			scrollSensitivity: 30,
			scrollSpeed: 10,
			draggable: /[uo]l/i.test(el.nodeName) ? 'li' : '>*',
			ghostClass: 'sortable-ghost',
			ignore: 'a, img',
			filter: null,
			animation: 0,
			setData: function (dataTransfer, dragEl) {
				dataTransfer.setData('Text', dragEl.textContent);
			},
			dropBubble: false,
			dragoverBubble: false,
			dataIdAttr: 'data-id',
			delay: 0
		};


		// Set default options
		for (var name in defaults) {
			!(name in options) && (options[name] = defaults[name]);
		}


		var group = options.group;

		if (!group || typeof group != 'object') {
			group = options.group = { name: group };
		}


		['pull', 'put'].forEach(function (key) {
			if (!(key in group)) {
				group[key] = true;
			}
		});


		options.groups = ' ' + group.name + (group.put.join ? ' ' + group.put.join(' ') : '') + ' ';


		// Bind all private methods
		for (var fn in this) {
			if (fn.charAt(0) === '_') {
				this[fn] = _bind(this, this[fn]);
			}
		}


		// Bind events
		_on(el, 'mousedown', this._onTapStart);
		_on(el, 'touchstart', this._onTapStart);

		_on(el, 'dragover', this);
		_on(el, 'dragenter', this);

		touchDragOverListeners.push(this._onDragOver);

		// Restore sorting
		options.store && this.sort(options.store.get(this));
	}


	Sortable.prototype = /** @lends Sortable.prototype */ {
		constructor: Sortable,

		_onTapStart: function (/** Event|TouchEvent */evt) {
			var _this = this,
				el = this.el,
				options = this.options,
				type = evt.type,
				touch = evt.touches && evt.touches[0],
				target = (touch || evt).target,
				originalTarget = target,
				filter = options.filter;


			if (type === 'mousedown' && evt.button !== 0 || options.disabled) {
				return; // only left button or enabled
			}

			target = _closest(target, options.draggable, el);

			if (!target) {
				return;
			}

			// get the index of the dragged element within its parent
			oldIndex = _index(target);

			// Check filter
			if (typeof filter === 'function') {
				if (filter.call(this, evt, target, this)) {
					_dispatchEvent(_this, originalTarget, 'filter', target, el, oldIndex);
					evt.preventDefault();
					return; // cancel dnd
				}
			}
			else if (filter) {
				filter = filter.split(',').some(function (criteria) {
					criteria = _closest(originalTarget, criteria.trim(), el);

					if (criteria) {
						_dispatchEvent(_this, criteria, 'filter', target, el, oldIndex);
						return true;
					}
				});

				if (filter) {
					evt.preventDefault();
					return; // cancel dnd
				}
			}


			if (options.handle && !_closest(originalTarget, options.handle, el)) {
				return;
			}


			// Prepare `dragstart`
			this._prepareDragStart(evt, touch, target);
		},

		_prepareDragStart: function (/** Event */evt, /** Touch */touch, /** HTMLElement */target) {
			var _this = this,
				el = _this.el,
				options = _this.options,
				ownerDocument = el.ownerDocument,
				dragStartFn;

			if (target && !dragEl && (target.parentNode === el)) {
				tapEvt = evt;

				rootEl = el;
				dragEl = target;
				nextEl = dragEl.nextSibling;
				activeGroup = options.group;

				dragStartFn = function () {
					// Delayed drag has been triggered
					// we can re-enable the events: touchmove/mousemove
					_this._disableDelayedDrag();

					// Make the element draggable
					dragEl.draggable = true;

					// Disable "draggable"
					options.ignore.split(',').forEach(function (criteria) {
						_find(dragEl, criteria.trim(), _disableDraggable);
					});

					// Bind the events: dragstart/dragend
					_this._triggerDragStart(touch);
				};

				_on(ownerDocument, 'mouseup', _this._onDrop);
				_on(ownerDocument, 'touchend', _this._onDrop);
				_on(ownerDocument, 'touchcancel', _this._onDrop);

				if (options.delay) {
					// If the user moves the pointer before the delay has been reached:
					// disable the delayed drag
					_on(ownerDocument, 'mousemove', _this._disableDelayedDrag);
					_on(ownerDocument, 'touchmove', _this._disableDelayedDrag);

					_this._dragStartTimer = setTimeout(dragStartFn, options.delay);
				} else {
					dragStartFn();
				}
			}
		},

		_disableDelayedDrag: function () {
			var ownerDocument = this.el.ownerDocument;

			clearTimeout(this._dragStartTimer);

			_off(ownerDocument, 'mousemove', this._disableDelayedDrag);
			_off(ownerDocument, 'touchmove', this._disableDelayedDrag);
		},

		_triggerDragStart: function (/** Touch */touch) {
			if (touch) {
				// Touch device support
				tapEvt = {
					target: dragEl,
					clientX: touch.clientX,
					clientY: touch.clientY
				};

				this._onDragStart(tapEvt, 'touch');
			}
			else if (!supportDraggable) {
				this._onDragStart(tapEvt, true);
			}
			else {
				_on(dragEl, 'dragend', this);
				_on(rootEl, 'dragstart', this._onDragStart);
			}

			try {
				if (document.selection) {
					document.selection.empty();
				} else {
					window.getSelection().removeAllRanges();
				}
			} catch (err) {
			}
		},

		_dragStarted: function () {
			if (rootEl && dragEl) {
				// Apply effect
				_toggleClass(dragEl, this.options.ghostClass, true);

				Sortable.active = this;

				// Drag start event
				_dispatchEvent(this, rootEl, 'start', dragEl, rootEl, oldIndex);
			}
		},

		_emulateDragOver: function () {
			if (touchEvt) {
				_css(ghostEl, 'display', 'none');

				var target = document.elementFromPoint(touchEvt.clientX, touchEvt.clientY),
					parent = target,
					groupName = ' ' + this.options.group.name + '',
					i = touchDragOverListeners.length;

				if (parent) {
					do {
						if (parent[expando] && parent[expando].options.groups.indexOf(groupName) > -1) {
							while (i--) {
								touchDragOverListeners[i]({
									clientX: touchEvt.clientX,
									clientY: touchEvt.clientY,
									target: target,
									rootEl: parent
								});
							}

							break;
						}

						target = parent; // store last element
					}
					/* jshint boss:true */
					while (parent = parent.parentNode);
				}

				_css(ghostEl, 'display', '');
			}
		},


		_onTouchMove: function (/**TouchEvent*/evt) {
			if (tapEvt) {
				var touch = evt.touches ? evt.touches[0] : evt,
					dx = touch.clientX - tapEvt.clientX,
					dy = touch.clientY - tapEvt.clientY,
					translate3d = evt.touches ? 'translate3d(' + dx + 'px,' + dy + 'px,0)' : 'translate(' + dx + 'px,' + dy + 'px)';

				touchEvt = touch;

				_css(ghostEl, 'webkitTransform', translate3d);
				_css(ghostEl, 'mozTransform', translate3d);
				_css(ghostEl, 'msTransform', translate3d);
				_css(ghostEl, 'transform', translate3d);

				evt.preventDefault();
			}
		},


		_onDragStart: function (/**Event*/evt, /**boolean*/useFallback) {
			var dataTransfer = evt.dataTransfer,
				options = this.options;

			this._offUpEvents();

			if (activeGroup.pull == 'clone') {
				cloneEl = dragEl.cloneNode(true);
				_css(cloneEl, 'display', 'none');
				rootEl.insertBefore(cloneEl, dragEl);
			}

			if (useFallback) {
				var rect = dragEl.getBoundingClientRect(),
					css = _css(dragEl),
					ghostRect;

				ghostEl = dragEl.cloneNode(true);

				_css(ghostEl, 'top', rect.top - parseInt(css.marginTop, 10));
				_css(ghostEl, 'left', rect.left - parseInt(css.marginLeft, 10));
				_css(ghostEl, 'width', rect.width);
				_css(ghostEl, 'height', rect.height);
				_css(ghostEl, 'opacity', '0.8');
				_css(ghostEl, 'position', 'fixed');
				_css(ghostEl, 'zIndex', '100000');

				rootEl.appendChild(ghostEl);

				// Fixing dimensions.
				ghostRect = ghostEl.getBoundingClientRect();
				_css(ghostEl, 'width', rect.width * 2 - ghostRect.width);
				_css(ghostEl, 'height', rect.height * 2 - ghostRect.height);

				if (useFallback === 'touch') {
					// Bind touch events
					_on(document, 'touchmove', this._onTouchMove);
					_on(document, 'touchend', this._onDrop);
					_on(document, 'touchcancel', this._onDrop);
				} else {
					// Old brwoser
					_on(document, 'mousemove', this._onTouchMove);
					_on(document, 'mouseup', this._onDrop);
				}

				this._loopId = setInterval(this._emulateDragOver, 150);
			}
			else {
				if (dataTransfer) {
					dataTransfer.effectAllowed = 'move';
					options.setData && options.setData.call(this, dataTransfer, dragEl);
				}

				_on(document, 'drop', this);
			}

			setTimeout(this._dragStarted, 0);
		},

		_onDragOver: function (/**Event*/evt) {
			var el = this.el,
				target,
				dragRect,
				revert,
				options = this.options,
				group = options.group,
				groupPut = group.put,
				isOwner = (activeGroup === group),
				canSort = options.sort;

			if (evt.preventDefault !== void 0) {
				evt.preventDefault();
				!options.dragoverBubble && evt.stopPropagation();
			}

			if (activeGroup && !options.disabled &&
				(isOwner
					? canSort || (revert = !rootEl.contains(dragEl))
					: activeGroup.pull && groupPut && (
						(activeGroup.name === group.name) || // by Name
						(groupPut.indexOf && ~groupPut.indexOf(activeGroup.name)) // by Array
					)
				) &&
				(evt.rootEl === void 0 || evt.rootEl === this.el)
			) {
				// Smart auto-scrolling
				_autoScroll(evt, options, this.el);

				if (_silent) {
					return;
				}

				target = _closest(evt.target, options.draggable, el);
				dragRect = dragEl.getBoundingClientRect();


				if (revert) {
					_cloneHide(true);

					if (cloneEl || nextEl) {
						rootEl.insertBefore(dragEl, cloneEl || nextEl);
					}
					else if (!canSort) {
						rootEl.appendChild(dragEl);
					}

					return;
				}


				if ((el.children.length === 0) || (el.children[0] === ghostEl) ||
					(el === evt.target) && (target = _ghostInBottom(el, evt))
				) {
					if (target) {
						if (target.animated) {
							return;
						}
						targetRect = target.getBoundingClientRect();
					}

					_cloneHide(isOwner);

					el.appendChild(dragEl);
					this._animate(dragRect, dragEl);
					target && this._animate(targetRect, target);
				}
				else if (target && !target.animated && target !== dragEl && (target.parentNode[expando] !== void 0)) {
					if (lastEl !== target) {
						lastEl = target;
						lastCSS = _css(target);
					}


					var targetRect = target.getBoundingClientRect(),
						width = targetRect.right - targetRect.left,
						height = targetRect.bottom - targetRect.top,
						floating = /left|right|inline/.test(lastCSS.cssFloat + lastCSS.display),
						isWide = (target.offsetWidth > dragEl.offsetWidth),
						isLong = (target.offsetHeight > dragEl.offsetHeight),
						halfway = (floating ? (evt.clientX - targetRect.left) / width : (evt.clientY - targetRect.top) / height) > 0.5,
						nextSibling = target.nextElementSibling,
						after
					;

					_silent = true;
					setTimeout(_unsilent, 30);

					_cloneHide(isOwner);

					if (floating) {
						after = (target.previousElementSibling === dragEl) && !isWide || halfway && isWide;
					} else {
						after = (nextSibling !== dragEl) && !isLong || halfway && isLong;
					}

					if (after && !nextSibling) {
						el.appendChild(dragEl);
					} else {
						target.parentNode.insertBefore(dragEl, after ? nextSibling : target);
					}

					this._animate(dragRect, dragEl);
					this._animate(targetRect, target);
				}
			}
		},

		_animate: function (prevRect, target) {
			var ms = this.options.animation;

			if (ms) {
				var currentRect = target.getBoundingClientRect();

				_css(target, 'transition', 'none');
				_css(target, 'transform', 'translate3d('
					+ (prevRect.left - currentRect.left) + 'px,'
					+ (prevRect.top - currentRect.top) + 'px,0)'
				);

				target.offsetWidth; // repaint

				_css(target, 'transition', 'all ' + ms + 'ms');
				_css(target, 'transform', 'translate3d(0,0,0)');

				clearTimeout(target.animated);
				target.animated = setTimeout(function () {
					_css(target, 'transition', '');
					_css(target, 'transform', '');
					target.animated = false;
				}, ms);
			}
		},

		_offUpEvents: function () {
			var ownerDocument = this.el.ownerDocument;

			_off(document, 'touchmove', this._onTouchMove);
			_off(ownerDocument, 'mouseup', this._onDrop);
			_off(ownerDocument, 'touchend', this._onDrop);
			_off(ownerDocument, 'touchcancel', this._onDrop);
		},

		_onDrop: function (/**Event*/evt) {
			var el = this.el,
				options = this.options;

			clearInterval(this._loopId);
			clearInterval(autoScroll.pid);

			clearTimeout(this.dragStartTimer);

			// Unbind events
			_off(document, 'drop', this);
			_off(document, 'mousemove', this._onTouchMove);
			_off(el, 'dragstart', this._onDragStart);

			this._offUpEvents();

			if (evt) {
				evt.preventDefault();
				!options.dropBubble && evt.stopPropagation();

				ghostEl && ghostEl.parentNode.removeChild(ghostEl);

				if (dragEl) {
					_off(dragEl, 'dragend', this);

					_disableDraggable(dragEl);
					_toggleClass(dragEl, this.options.ghostClass, false);

					if (rootEl !== dragEl.parentNode) {
						newIndex = _index(dragEl);

						// drag from one list and drop into another
						_dispatchEvent(null, dragEl.parentNode, 'sort', dragEl, rootEl, oldIndex, newIndex);
						_dispatchEvent(this, rootEl, 'sort', dragEl, rootEl, oldIndex, newIndex);

						// Add event
						_dispatchEvent(null, dragEl.parentNode, 'add', dragEl, rootEl, oldIndex, newIndex);

						// Remove event
						_dispatchEvent(this, rootEl, 'remove', dragEl, rootEl, oldIndex, newIndex);
					}
					else {
						// Remove clone
						cloneEl && cloneEl.parentNode.removeChild(cloneEl);

						if (dragEl.nextSibling !== nextEl) {
							// Get the index of the dragged element within its parent
							newIndex = _index(dragEl);

							// drag & drop within the same list
							_dispatchEvent(this, rootEl, 'update', dragEl, rootEl, oldIndex, newIndex);
							_dispatchEvent(this, rootEl, 'sort', dragEl, rootEl, oldIndex, newIndex);
						}
					}

					// Drag end event
					Sortable.active && _dispatchEvent(this, rootEl, 'end', dragEl, rootEl, oldIndex, newIndex);
				}

				// Nulling
				rootEl =
				dragEl =
				ghostEl =
				nextEl =
				cloneEl =

				scrollEl =
				scrollParentEl =

				tapEvt =
				touchEvt =

				lastEl =
				lastCSS =

				activeGroup =
				Sortable.active = null;

				// Save sorting
				this.save();
			}
		},


		handleEvent: function (/**Event*/evt) {
			var type = evt.type;

			if (type === 'dragover' || type === 'dragenter') {
				if (dragEl) {
					this._onDragOver(evt);
					_globalDragOver(evt);
				}
			}
			else if (type === 'drop' || type === 'dragend') {
				this._onDrop(evt);
			}
		},


		/**
		 * Serializes the item into an array of string.
		 * @returns {String[]}
		 */
		toArray: function () {
			var order = [],
				el,
				children = this.el.children,
				i = 0,
				n = children.length,
				options = this.options;

			for (; i < n; i++) {
				el = children[i];
				if (_closest(el, options.draggable, this.el)) {
					order.push(el.getAttribute(options.dataIdAttr) || _generateId(el));
				}
			}

			return order;
		},


		/**
		 * Sorts the elements according to the array.
		 * @param  {String[]}  order  order of the items
		 */
		sort: function (order) {
			var items = {}, rootEl = this.el;

			this.toArray().forEach(function (id, i) {
				var el = rootEl.children[i];

				if (_closest(el, this.options.draggable, rootEl)) {
					items[id] = el;
				}
			}, this);

			order.forEach(function (id) {
				if (items[id]) {
					rootEl.removeChild(items[id]);
					rootEl.appendChild(items[id]);
				}
			});
		},


		/**
		 * Save the current sorting
		 */
		save: function () {
			var store = this.options.store;
			store && store.set(this);
		},


		/**
		 * For each element in the set, get the first element that matches the selector by testing the element itself and traversing up through its ancestors in the DOM tree.
		 * @param   {HTMLElement}  el
		 * @param   {String}       [selector]  default: `options.draggable`
		 * @returns {HTMLElement|null}
		 */
		closest: function (el, selector) {
			return _closest(el, selector || this.options.draggable, this.el);
		},


		/**
		 * Set/get option
		 * @param   {string} name
		 * @param   {*}      [value]
		 * @returns {*}
		 */
		option: function (name, value) {
			var options = this.options;

			if (value === void 0) {
				return options[name];
			} else {
				options[name] = value;
			}
		},


		/**
		 * Destroy
		 */
		destroy: function () {
			var el = this.el;

			el[expando] = null;

			_off(el, 'mousedown', this._onTapStart);
			_off(el, 'touchstart', this._onTapStart);

			_off(el, 'dragover', this);
			_off(el, 'dragenter', this);

			// Remove draggable attributes
			Array.prototype.forEach.call(el.querySelectorAll('[draggable]'), function (el) {
				el.removeAttribute('draggable');
			});

			touchDragOverListeners.splice(touchDragOverListeners.indexOf(this._onDragOver), 1);

			this._onDrop();

			this.el = el = null;
		}
	};


	function _cloneHide(state) {
		if (cloneEl && (cloneEl.state !== state)) {
			_css(cloneEl, 'display', state ? 'none' : '');
			!state && cloneEl.state && rootEl.insertBefore(cloneEl, dragEl);
			cloneEl.state = state;
		}
	}


	function _bind(ctx, fn) {
		var args = slice.call(arguments, 2);
		return	fn.bind ? fn.bind.apply(fn, [ctx].concat(args)) : function () {
			return fn.apply(ctx, args.concat(slice.call(arguments)));
		};
	}


	function _closest(/**HTMLElement*/el, /**String*/selector, /**HTMLElement*/ctx) {
		if (el) {
			ctx = ctx || document;
			selector = selector.split('.');

			var tag = selector.shift().toUpperCase(),
				re = new RegExp('\\s(' + selector.join('|') + ')\\s', 'g');

			do {
				if (
					(tag === '>*' && el.parentNode === ctx) || (
						(tag === '' || el.nodeName.toUpperCase() == tag) &&
						(!selector.length || ((' ' + el.className + ' ').match(re) || []).length == selector.length)
					)
				) {
					return el;
				}
			}
			while (el !== ctx && (el = el.parentNode));
		}

		return null;
	}


	function _globalDragOver(/**Event*/evt) {
		evt.dataTransfer.dropEffect = 'move';
		evt.preventDefault();
	}


	function _on(el, event, fn) {
		el.addEventListener(event, fn, false);
	}


	function _off(el, event, fn) {
		el.removeEventListener(event, fn, false);
	}


	function _toggleClass(el, name, state) {
		if (el) {
			if (el.classList) {
				el.classList[state ? 'add' : 'remove'](name);
			}
			else {
				var className = (' ' + el.className + ' ').replace(RSPACE, ' ').replace(' ' + name + ' ', ' ');
				el.className = (className + (state ? ' ' + name : '')).replace(RSPACE, ' ');
			}
		}
	}


	function _css(el, prop, val) {
		var style = el && el.style;

		if (style) {
			if (val === void 0) {
				if (document.defaultView && document.defaultView.getComputedStyle) {
					val = document.defaultView.getComputedStyle(el, '');
				}
				else if (el.currentStyle) {
					val = el.currentStyle;
				}

				return prop === void 0 ? val : val[prop];
			}
			else {
				if (!(prop in style)) {
					prop = '-webkit-' + prop;
				}

				style[prop] = val + (typeof val === 'string' ? '' : 'px');
			}
		}
	}


	function _find(ctx, tagName, iterator) {
		if (ctx) {
			var list = ctx.getElementsByTagName(tagName), i = 0, n = list.length;

			if (iterator) {
				for (; i < n; i++) {
					iterator(list[i], i);
				}
			}

			return list;
		}

		return [];
	}


	function _disableDraggable(el) {
		el.draggable = false;
	}


	function _unsilent() {
		_silent = false;
	}


	/** @returns {HTMLElement|false} */
	function _ghostInBottom(el, evt) {
		var lastEl = el.lastElementChild, rect = lastEl.getBoundingClientRect();
		return (evt.clientY - (rect.top + rect.height) > 5) && lastEl; // min delta
	}


	/**
	 * Generate id
	 * @param   {HTMLElement} el
	 * @returns {String}
	 * @private
	 */
	function _generateId(el) {
		var str = el.tagName + el.className + el.src + el.href + el.textContent,
			i = str.length,
			sum = 0;

		while (i--) {
			sum += str.charCodeAt(i);
		}

		return sum.toString(36);
	}

	/**
	 * Returns the index of an element within its parent
	 * @param el
	 * @returns {number}
	 * @private
	 */
	function _index(/**HTMLElement*/el) {
		var index = 0;
		while (el && (el = el.previousElementSibling)) {
			if (el.nodeName.toUpperCase() !== 'TEMPLATE') {
				index++;
			}
		}
		return index;
	}

	function _throttle(callback, ms) {
		var args, _this;

		return function () {
			if (args === void 0) {
				args = arguments;
				_this = this;

				setTimeout(function () {
					if (args.length === 1) {
						callback.call(_this, args[0]);
					} else {
						callback.apply(_this, args);
					}

					args = void 0;
				}, ms);
			}
		};
	}

	function _extend(dst, src) {
		if (dst && src) {
			for (var key in src) {
				if (src.hasOwnProperty(key)) {
					dst[key] = src[key];
				}
			}
		}

		return dst;
	}


	// Export utils
	Sortable.utils = {
		on: _on,
		off: _off,
		css: _css,
		find: _find,
		bind: _bind,
		is: function (el, selector) {
			return !!_closest(el, selector, el);
		},
		extend: _extend,
		throttle: _throttle,
		closest: _closest,
		toggleClass: _toggleClass,
		index: _index
	};


	Sortable.version = '1.2.0';


	/**
	 * Create sortable instance
	 * @param {HTMLElement}  el
	 * @param {Object}      [options]
	 */
	Sortable.create = function (el, options) {
		return new Sortable(el, options);
	};

	// Export
	return Sortable;
});

var concourse = {
  redirect: function(href) {
    window.location = href;
  }
};

$(".js-expandable").on("click", function() {
  if($(this).parent().hasClass("expanded")) {
    $(this).parent().removeClass("expanded");
  } else {
    $(this).parent().addClass("expanded");
  }
});

concourse.Build = function ($el) {
  this.$el = $el;
  this.$abortBtn = this.$el.find('.js-abortBuild');
  this.buildID = this.$el.data('build-id');
  this.abortEndpoint = '/api/v1/builds/' + this.buildID + '/abort';
};

concourse.Build.prototype.bindEvents = function () {
  var _this = this;
  this.$abortBtn.on('click', function(event) {
    _this.abort();
  });
};

concourse.Build.prototype.abort = function() {
  var _this = this;

  $.ajax({
    method: 'POST',
    url: _this.abortEndpoint
  }).done(function (resp, jqxhr) {
    _this.$abortBtn.remove();
  }).error(function (resp) {
    _this.$abortBtn.addClass('errored');

    if (resp.status == 401) {
      concourse.redirect("/login");
    }
  });
};

$(function () {
  if ($('.js-build').length) {
    var build = new concourse.Build($('.js-build'));
    build.bindEvents();
  }
});

// <button class="btn-pause disabled js-pauseResourceCheck"><i class="fa fa-fw fa-pause"></i></button>

(function ($) {
    $.fn.pausePlayBtn = function () {
      var $el = $(this);
      return {
        loading: function() {
          $el.removeClass('disabled enabled').addClass('loading');
          $el.find('i').removeClass('fa-pause').addClass('fa-circle-o-notch fa-spin');
        },

        enable: function() {
          $el.removeClass('loading').addClass('enabled');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-play');
        },

        error: function() {
          $el.removeClass('loading').addClass('errored');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-pause');
        },

        disable: function() {
          $el.removeClass('loading').addClass('disabled');
          $el.find('i').removeClass('fa-circle-o-notch fa-spin').addClass('fa-pause');
        }
      };
    };
})(jQuery);

$(function () {
  if ($('.js-job').length) {
    var pauseUnpause = new concourse.PauseUnpause($('.js-job'));
    pauseUnpause.bindEvents();

		$('.js-build').each(function(i, el){
			var startTime, endTime,
				$build = $(el),
				status = $build.data('status'),
				$buildTimes = $build.find(".js-build-times"),
				start = $buildTimes.data('start-time'),
				end = $buildTimes.data('end-time'),
				$startTime = $("<time>"),
				$endTime = $("<time>");

			if(window.moment === undefined){
				console.log("moment library not included, cannot parse durations");
				return;
			}

			if (start > 0) {
				startTime = moment.unix(start);
				$startTime.text(startTime.fromNow());
				$startTime.attr("datetime", startTime.format());
				$startTime.attr("title", startTime.format("lll Z"));
				$("<div/>").text("started: ").append($startTime).appendTo($buildTimes);
			}

			endTime = moment.unix(end);
			$endTime.text(endTime.fromNow());
			$endTime.attr("datetime", endTime.format());
			$endTime.attr("title", endTime.format("lll Z"));
			$("<div/>").text(status + ": ").append($endTime).appendTo($buildTimes);

			if (end > 0 && start > 0) {
				var duration = moment.duration(endTime.diff(startTime));

				var durationEle = $("<span>");
				durationEle.addClass("duration");
				durationEle.text(duration.format("h[h]m[m]s[s]"));

				$("<div/>").text("duration: ").append(durationEle).appendTo($buildTimes);
			}
		});
	}
});

concourse.PauseUnpause = function ($el, pauseCallback, unpauseCallback) {
  this.$el = $el;
  this.pauseCallback = pauseCallback === undefined ? function(){} : pauseCallback;
  this.unpauseCallback = unpauseCallback === undefined ? function(){} : unpauseCallback;
  this.pauseBtn = this.$el.find('.js-pauseUnpause').pausePlayBtn();
  this.pauseEndpoint = this.$el.data('endpoint') + "/pause";
  this.unPauseEndpoint = this.$el.data('endpoint') + "/unpause";
  this.teamName = this.$el.data('teamname');
};

concourse.PauseUnpause.prototype.bindEvents = function () {
  var _this = this;

  _this.$el.delegate('.js-pauseUnpause.disabled', 'click', function (event) {
    _this.pause();
  });

  _this.$el.delegate('.js-pauseUnpause.enabled', 'click', function (event) {
    _this.unpause();
  });
};

concourse.PauseUnpause.prototype.pause = function (pause) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.pauseEndpoint,
  }).done(function (resp, jqxhr) {
    _this.pauseBtn.enable();
    _this.pauseCallback();
  }).error(function (resp) {
    _this.requestError(resp);
  });
};


concourse.PauseUnpause.prototype.unpause = function (event) {
  var _this = this;
  _this.pauseBtn.loading();

  $.ajax({
    method: 'PUT',
    url: _this.unPauseEndpoint
  }).done(function (resp) {
    _this.pauseBtn.disable();
    _this.unpauseCallback();
  }).error(function (resp) {
    _this.requestError(resp);
  });
};

concourse.PauseUnpause.prototype.requestError = function (resp) {
  this.pauseBtn.error();

  if (resp.status == 401) {
    concourse.redirect("/teams/" + this.teamName + "/login");
  }
};

(function(sortable){
  concourse.PipelinesNav = function ($el) {
    this.$el = $($el);
    this.$toggle = $el.find($('.js-sidebar-toggle'));
    this.$list = $el.find($('.js-pipelines-list'));
    this.pipelinesEndpoint = '/api/v1/pipelines';
  };

  concourse.PipelinesNav.prototype.bindEvents = function () {
    console.log("toggle", this.$toggle);
    var _this = this;
    _this.$toggle.on("click", function() {
        _this.toggle();
    });

    sortable.create(_this.$list[0], {
      "onUpdate": function() {
        _this.onSort();
      }
    });

    _this.loadPipelines();
  };

  concourse.PipelinesNav.prototype.onSort = function() {
    var _this = this;

    var pipelineNames = _this.$list.find('a')
      .toArray()
      .map(function(e) {
        return e.innerHTML;
      });

    var teamName = $(_this.$list[0]).find('.js-pauseUnpause').parent().data('teamName');

    $.ajax({
      method: 'PUT',
      url: '/api/v1/teams/' + teamName + '/pipelines/ordering',
      contentType: "application/json",
      data: JSON.stringify(pipelineNames)
    });
  };

  concourse.PipelinesNav.prototype.toggle = function() {
    $('.js-sidebar').toggleClass('visible');
  };

  concourse.PipelinesNav.prototype.loadPipelines = function() {
    var _this = this;
    $.ajax({
      method: 'GET',
      url: _this.pipelinesEndpoint
    }).done(function(resp, jqxhr){
      $(resp).each( function(index, pipeline){
        var $pipelineListItem = $("<li>");

        var ed = pipeline.paused ? 'enabled' : 'disabled';
        var icon = pipeline.paused ? 'play' : 'pause';

        $pipelineListItem.html('<span class="btn-pause fl ' + ed + ' js-pauseUnpause"><i class="fa fa-fw fa-' + icon +  '"></i></span><a href="' + pipeline.url + '">' + pipeline.name + '</a>');
        $pipelineListItem.data('endpoint', '/api/v1/teams/' +  pipeline.team_name + '/pipelines/' + pipeline.name);
        $pipelineListItem.data('pipelineName', pipeline.name);
        $pipelineListItem.data('teamName', pipeline.team_name);
        $pipelineListItem.addClass('clearfix');


        _this.$list.append($pipelineListItem);

        _this.newPauseUnpause($pipelineListItem);

        if(concourse.pipelineName === pipeline.name && pipeline.paused) {
          _this.$el.find('.js-top-bar').addClass('paused');
        }
      });
    });
  };

  concourse.PipelinesNav.prototype.newPauseUnpause = function($el) {
    var _this = this;
    var pauseUnpause = new concourse.PauseUnpause($el, function() {
      if($el.data('pipelineName') === concourse.pipelineName) {
        _this.$el.find('.js-top-bar').addClass('paused');
      }
    }, function() {
      if($el.data('pipelineName') === concourse.pipelineName) {
        _this.$el.find('.js-top-bar').removeClass('paused');
      }
    });
    pauseUnpause.bindEvents();
  };
})(Sortable);

$(function () {
  if ($('.js-with-pipeline').length) {
    var withPipeline = new concourse.PipelinesNav($('.js-with-pipeline'));
    withPipeline.bindEvents();
  }
});

$(function () {
  if ($('.js-resource').length) {
    var pauseUnpause = new concourse.PauseUnpause(
      $('.js-resource'),
      function() {}, // on pause
      function() {}  // on unpause
    );
    pauseUnpause.bindEvents();
  }
});

(function(){
  concourse.StepData = function(data){
    if(data === undefined){
      this.data = {};
    } else {
      this.data = data;
    }
    this.idCounter = 1;
    this.parallelGroupStore = {};
    return this;
  };

  var stepDataProto = {
    updateIn: function(location, upsertFunction){
      var newData = jQuery.extend(true, {}, this.data);
      var keyPath;

      if(Array.isArray(location)){
        keyPath = location.join('.');
      } else {
        keyPath = location.id;
      }

      var before = newData[keyPath];
      newData[keyPath] = upsertFunction(newData[keyPath]);
      var after = newData[keyPath];

      if (before === after) {
        return this;
      }

      return new concourse.StepData(newData);
    },

    getIn: function(location) {
      if(Array.isArray(location)){
        return this.data[location.join('.')];
      } else {
        return this.data[location.id];
      }
    },

    setIn: function(location, val) {
      var newData = jQuery.extend(true, {}, this.data);

      if (Array.isArray(location)) {
        newData[location.join('.')] = val;
      }
      else {
        newData[location.id] = val;
      }
      return new concourse.StepData(newData);

    },

    forEach: function(cb) {
      for(var key in this.data) {
        cb(this.data[key]);
      }
    },

    getSorted: function() {
      var ret = [];
      for(var key in this.data) {
        ret.push([key, this.data[key]]);
      }

      ret = ret.sort(function(a, b){
        var aLoc = a[0].split('.'),
            bLoc = b[0].split('.');

        for(var i = 0; i < aLoc.length; i++){
          var aVal = parseInt(aLoc[i]);
          var bVal = parseInt(bLoc[i]);

          if(aVal > bVal){
            return 1;
          }
        }

        return -1;
      });

      ret = ret.map(function(val){
        return val[1];
      });

      return ret;
    },

    translateLocation: function(location, substep) {
      if (!Array.isArray(location)) {
        return location;
      }

      var id,
          parallel_group = 0,
          parent_id = 0;

      if(location.length > 1) {
        var parallelGroupLocation = location.slice(0, location.length - 1).join('.');

        if(this.parallelGroupStore[parallelGroupLocation] === undefined){
          this.parallelGroupStore[parallelGroupLocation] = this.idCounter;
          this.idCounter++;
        }

        parallel_group = this.parallelGroupStore[parallelGroupLocation];

        if(location.length > 2) {
          var parentGroupLocation = location.slice(0, location.length - 2).join('.');

          if(this.parallelGroupStore[parentGroupLocation] === undefined){
            parent_id = 0;
          } else {
            parent_id = this.parallelGroupStore[parentGroupLocation];
          }
        }
      }


      id = this.idCounter;
      this.idCounter++;

      if(substep){
        parent_id = id - 1;
        parallel_group = 0;
      }


      return {
        id: id,
        parallel_group: parallel_group,
        parent_id: parent_id
      };
    },

    getRenderableData: function() {
      var _this = this,
          ret = [],
          allObjects = [],
          sortedData = _this.getSorted();

      var addStepToGroup = function(primaryGroupID, secondaryGroupID, parentID, renderGroup){
        if(allObjects[primaryGroupID] === undefined){
          allObjects[primaryGroupID] = renderGroup;
        } else if (allObjects[primaryGroupID].hold) {
          renderGroup.groupSteps = allObjects[primaryGroupID].groupSteps;
          renderGroup.children = allObjects[primaryGroupID].children;
          allObjects[primaryGroupID] = renderGroup;
        }


        if(secondaryGroupID < primaryGroupID) {
          allObjects[primaryGroupID].groupSteps[location.id] = allObjects[location.id];
        }

        if (secondaryGroupID !== 0 && secondaryGroupID < primaryGroupID) {
          allObjects[secondaryGroupID].groupSteps[primaryGroupID] = allObjects[primaryGroupID];
        }

        if (parentID !== 0) {
          if(step.isHook()){
            allObjects[parentID].children[primaryGroupID] = allObjects[primaryGroupID];
          } else {
            allObjects[parentID].groupSteps[primaryGroupID] = allObjects[primaryGroupID];
          }
        }
      };

      for(var i = 0; i < sortedData.length; i++){
        var step = sortedData[i];
        var location = _this.translateLocation(step.origin().location, step.origin().substep);
        var stepLogs = step.logs();
        var logLines = stepLogs.lines;

        var render = {
          key: location.id,
          step: step,
          location: location,
          logLines: logLines,
          children: []
        };

        allObjects[location.id] = render;

        if (location.parent_id !== 0 && allObjects[location.parent_id] === undefined) {
          allObjects[location.parent_id] = {hold: true, groupSteps: [], children: []};
        }

        if (location.parallel_group !== 0 && allObjects[location.parallel_group] === undefined) {
          allObjects[location.parallel_group] = {hold: true, groupSteps: [], children: []};
        }

        location.serial_group = location.serial_group ? location.serial_group : 0;

        if (location.serial_group !== 0) {
          renderSerialGroup = {
            serial: true,
            step: step,
            location: location,
            key: location.serial_group,
            groupSteps: [],
            children: []
          };

          addStepToGroup(location.serial_group, location.parallel_group, location.parent_id, renderSerialGroup);
        }

        if(location.parallel_group !== 0) {
          renderParallelGroup = {
            aggregate: true,
            step: step,
            location: location,
            key: location.parallel_group,
            groupSteps: [],
            children: []
          };

          addStepToGroup(location.parallel_group, location.serial_group, location.parent_id, renderParallelGroup);
        }


        if (location.parallel_group !== 0 &&
          (location.serial_group === 0 || location.serial_group > location.parallel_group)
        ) {
          ret[location.parallel_group] = allObjects[location.parallel_group];
        } else if (location.serial_group !== 0) {
          ret[location.serial_group] = allObjects[location.serial_group];
        } else {
          ret[location.id] = allObjects[location.id];

          if(location.parent_id !== 0){
            allObjects[location.parent_id].children[location.id] = allObjects[location.id];
          }
        }
      }

      return ret;
    }

  };

  concourse.StepData.prototype = stepDataProto;
})();
