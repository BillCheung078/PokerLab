(() => {
  window.PokerLab = window.PokerLab || {};

  const eventTypes = [
    "game_started",
    "players_joined",
    "card_dealt",
    "bet_action",
    "community_cards",
    "hand_result",
  ];
  const maxFeedItems = 10;
  const maxSeenEvents = 128;

  function createSessionStreamManager(streamUrl) {
    return {
      streamUrl,
      source: null,
      reconnectTimer: null,
      subscribers: new Map(),
      nextSubscriberID: 0,
      connectionState: "connecting",
      connectionDetail: "Opening stream...",
      subscribe(tableId, handlers) {
        const id = "sub_" + (++this.nextSubscriberID);
        const subscriber = {
          tableId,
          onEvent: handlers.onEvent,
          onState: handlers.onState,
        };

        this.subscribers.set(id, subscriber);
        this.notifySubscriberState(subscriber);

        if (this.activeSubscriberCount() === 1) {
          this.openStream();
        } else if (this.source) {
          this.refresh();
        }

        return () => {
          this.subscribers.delete(id);
          if (this.activeSubscriberCount() === 0) {
            this.destroy();
          }
        };
      },
      activeSubscriberCount() {
        return this.subscribers.size;
      },
      refresh() {
        if (this.activeSubscriberCount() === 0) {
          this.destroy();
          return;
        }

        this.openStream();
      },
      openStream() {
        if (typeof window.EventSource === "undefined") {
          this.setConnectionState("disconnected", "EventSource is not supported in this browser.");
          return;
        }

        this.clearReconnectTimer();
        this.closeSource();

        this.setConnectionState(
          this.connectionState === "live" ? "reconnecting" : "connecting",
          this.connectionState === "live" ? "Refreshing shared stream..." : "Opening shared stream..."
        );

        const source = new window.EventSource(this.streamUrl);
        this.source = source;

        source.onopen = () => {
          if (this.source !== source) {
            return;
          }
          this.setConnectionState("live", "Shared live updates connected.");
        };

        source.onerror = () => {
          if (this.source !== source) {
            return;
          }

          if (source.readyState === window.EventSource.CONNECTING) {
            this.setConnectionState("reconnecting", "Attempting to reconnect shared stream...");
            return;
          }

          this.setConnectionState("disconnected", "Shared stream unavailable. Retrying shortly.");
          this.scheduleReconnect();
        };

        eventTypes.forEach((eventType) => {
          source.addEventListener(eventType, (message) => {
            if (this.source !== source) {
              return;
            }

            const event = parseEventPayload(message, eventType);
            if (!event || !event.tableId) {
              return;
            }

            for (const subscriber of this.subscribers.values()) {
              if (subscriber.tableId !== event.tableId || typeof subscriber.onEvent !== "function") {
                continue;
              }
              subscriber.onEvent(event);
            }
          });
        });
      },
      destroy() {
        this.clearReconnectTimer();
        this.closeSource();
      },
      closeSource() {
        if (!this.source) {
          return;
        }

        this.source.close();
        this.source = null;
      },
      scheduleReconnect() {
        if (this.reconnectTimer !== null || this.activeSubscriberCount() === 0) {
          return;
        }

        this.reconnectTimer = window.setTimeout(() => {
          this.reconnectTimer = null;
          if (this.activeSubscriberCount() === 0) {
            return;
          }
          this.openStream();
        }, 2000);
      },
      clearReconnectTimer() {
        if (this.reconnectTimer === null) {
          return;
        }

        window.clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
      },
      setConnectionState(state, detail) {
        this.connectionState = state;
        this.connectionDetail = detail;

        for (const subscriber of this.subscribers.values()) {
          this.notifySubscriberState(subscriber);
        }
      },
      notifySubscriberState(subscriber) {
        if (typeof subscriber.onState !== "function") {
          return;
        }

        subscriber.onState({
          state: this.connectionState,
          detail: this.connectionDetail,
        });
      },
    };
  }

  function createTableStream(config) {
    return {
      tableId: config.tableId || "",
      status: config.initialStatus || "Runtime pending",
      gameNumber: Number(config.initialGameNumber || 0),
      blindLevel: config.initialBlindLevel || "",
      eventCount: Number(config.initialEventCount || 0),
      connectionState: "connecting",
      connectionDetail: "Opening shared stream...",
      events: [],
      unsubscribe: null,
      cleanupHandler: null,
      unloadHandler: null,
      seenEventIDs: [],
      seenEventLookup: Object.create(null),
      initialLastEventAt: parseTime(config.initialLastEventAt),
      init() {
        if (this.$el.dataset.streamInitialized === "true") {
          return;
        }
        this.$el.dataset.streamInitialized = "true";

        this.cleanupHandler = (browserEvent) => {
          const target = browserEvent.target;
          if (!(target instanceof Element)) {
            return;
          }
          if (target === this.$el || target.contains(this.$el)) {
            this.destroyStream();
          }
        };

        this.unloadHandler = () => {
          this.destroyStream();
        };

        document.body.addEventListener("htmx:beforeCleanupElement", this.cleanupHandler);
        window.addEventListener("beforeunload", this.unloadHandler);

        this.unsubscribe = window.PokerLab.sessionStream.subscribe(this.tableId, {
          onEvent: (event) => this.handleStreamEvent(event),
          onState: (snapshot) => {
            this.connectionState = snapshot.state;
            this.connectionDetail = snapshot.detail;
          },
        });
      },
      destroy() {
        this.destroyStream();
      },
      destroyStream() {
        if (typeof this.unsubscribe === "function") {
          this.unsubscribe();
          this.unsubscribe = null;
        }
        delete this.$el.dataset.streamInitialized;

        if (this.cleanupHandler) {
          document.body.removeEventListener("htmx:beforeCleanupElement", this.cleanupHandler);
          this.cleanupHandler = null;
        }
        if (this.unloadHandler) {
          window.removeEventListener("beforeunload", this.unloadHandler);
          this.unloadHandler = null;
        }
      },
      handleStreamEvent(event) {
        if (!event || this.hasSeenEvent(event.id)) {
          return;
        }

        this.rememberEvent(event.id);
        this.applyEvent(event);
      },
      hasSeenEvent(eventID) {
        return Boolean(eventID) && Boolean(this.seenEventLookup[eventID]);
      },
      rememberEvent(eventID) {
        if (!eventID || this.seenEventLookup[eventID]) {
          return;
        }

        this.seenEventLookup[eventID] = true;
        this.seenEventIDs.push(eventID);

        if (this.seenEventIDs.length <= maxSeenEvents) {
          return;
        }

        const oldest = this.seenEventIDs.shift();
        if (oldest) {
          delete this.seenEventLookup[oldest];
        }
      },
      applyEvent(event) {
        this.status = statusLabelForType(event.type);

        if (event.type === "game_started") {
          this.gameNumber = asNumber(event.payload.game_number, this.gameNumber);
          this.blindLevel = asString(event.payload.blind_level) || this.blindLevel;
        }

        if (shouldCountEvent(event, this.initialLastEventAt)) {
          this.eventCount += 1;
        }

        this.events.unshift({
          id: event.id,
          title: eventTitle(event),
          detail: eventDetail(event),
          timeLabel: formatTime(event.at),
        });
        this.events = this.events.slice(0, maxFeedItems);
      },
      connectionLabel() {
        switch (this.connectionState) {
          case "live":
            return "Live";
          case "reconnecting":
            return "Reconnecting";
          case "disconnected":
            return "Offline";
          default:
            return "Connecting";
        }
      },
      connectionClass() {
        return "stream-pill-" + this.connectionState;
      },
      gameLabel() {
        return this.gameNumber > 0 ? "#" + this.gameNumber : "Pending";
      },
      blindLabel() {
        return this.blindLevel || "Pending";
      },
      feedHint() {
        if (this.events.length === 0) {
          return "Waiting for stream";
        }
        return "Latest " + this.events.length + " events";
      },
    };
  }

  function registerAlpineData() {
    if (!window.Alpine || window.Alpine.__pokerLabTableStreamRegistered) {
      return;
    }

    window.Alpine.data("tableStream", (config) => createTableStream(config));
    window.Alpine.__pokerLabTableStreamRegistered = true;
  }

  window.PokerLab.sessionStream = createSessionStreamManager("/stream");
  window.PokerLab.tableStream = createTableStream;

  document.addEventListener("alpine:init", registerAlpineData);

  if (window.Alpine) {
    registerAlpineData();
  }

  document.body.addEventListener("htmx:afterSwap", (browserEvent) => {
    if (!window.Alpine || typeof window.Alpine.initTree !== "function") {
      return;
    }

    const target = browserEvent.target;
    if (!(target instanceof Element)) {
      return;
    }

    window.Alpine.initTree(target);
  });

  function parseEventPayload(message, fallbackType) {
    let parsed;
    try {
      parsed = JSON.parse(message.data);
    } catch (_error) {
      return null;
    }

    const eventType = asString(parsed.type) || fallbackType;
    return {
      id: asString(parsed.id) || asString(message.lastEventId),
      tableId: asString(parsed.table_id),
      type: eventType,
      at: parseTime(parsed.at),
      payload: parsed.payload && typeof parsed.payload === "object" ? parsed.payload : {},
    };
  }

  function shouldCountEvent(event, initialLastEventAt) {
    if (!event.at) {
      return true;
    }
    if (!initialLastEventAt) {
      return true;
    }

    return event.at.getTime() > initialLastEventAt.getTime();
  }

  function eventTitle(event) {
    switch (event.type) {
      case "game_started":
        return "Hand started";
      case "players_joined":
        return "Players ready";
      case "card_dealt":
        return "Pocket cards dealt";
      case "bet_action":
        return "Betting action";
      case "community_cards":
        return communityStreetLabel(event.payload.street);
      case "hand_result":
        return "Hand result";
      default:
        return humanizeToken(event.type || "table_update");
    }
  }

  function eventDetail(event) {
    const payload = event.payload || {};

    switch (event.type) {
      case "game_started":
        return "Hand #" + asNumber(payload.game_number, 0) + " at blinds " + (asString(payload.blind_level) || "pending");
      case "players_joined":
        return asNumber(payload.player_count, 0) + " players seated";
      case "card_dealt":
        return "Seat " + asNumber(payload.seat, 0) + " received " + joinCards(payload.cards);
      case "bet_action":
        return "Seat " + asNumber(payload.seat, 0) + " " + humanizeToken(asString(payload.action) || "acted") + amountSuffix(payload.amount);
      case "community_cards":
        return communityStreetLabel(payload.street) + ": " + joinCards(payload.cards);
      case "hand_result":
        return asString(payload.summary) || "Hand finished";
      default:
        return "Table event received";
    }
  }

  function statusLabelForType(eventType) {
    switch (eventType) {
      case "game_started":
        return "Starting hand";
      case "players_joined":
        return "Players seated";
      case "card_dealt":
        return "Preflop cards dealt";
      case "bet_action":
        return "Betting in progress";
      case "community_cards":
        return "Board updated";
      case "hand_result":
        return "Hand complete";
      default:
        return "Runtime active";
    }
  }

  function communityStreetLabel(street) {
    switch (asString(street)) {
      case "flop":
        return "Flop";
      case "turn":
        return "Turn";
      case "river":
        return "River";
      default:
        return "Community cards";
    }
  }

  function formatTime(value) {
    if (!(value instanceof Date) || Number.isNaN(value.getTime())) {
      return "";
    }

    return value.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  }

  function joinCards(cards) {
    if (!Array.isArray(cards) || cards.length === 0) {
      return "cards";
    }

    return cards.map((card) => asString(card)).filter(Boolean).join(" ");
  }

  function amountSuffix(amount) {
    const numeric = asNumber(amount, 0);
    if (numeric <= 0) {
      return "";
    }

    return " for " + numeric;
  }

  function humanizeToken(value) {
    return asString(value)
      .replace(/_/g, " ")
      .replace(/\b\w/g, (char) => char.toUpperCase());
  }

  function asString(value) {
    return typeof value === "string" ? value : "";
  }

  function asNumber(value, fallback) {
    const numeric = Number(value);
    return Number.isFinite(numeric) ? numeric : fallback;
  }

  function parseTime(value) {
    const text = asString(value);
    if (!text) {
      return null;
    }

    const date = new Date(text);
    return Number.isNaN(date.getTime()) ? null : date;
  }
})();
