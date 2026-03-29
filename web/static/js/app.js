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
      actionLine: "Waiting for the next hand to begin.",
      players: defaultPlayers(),
      boardCards: [],
      betBursts: [],
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
        this.syncVisualState(event);

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
      syncVisualState(event) {
        const payload = event.payload || {};

        switch (event.type) {
          case "game_started":
            this.gameNumber = asNumber(payload.game_number, this.gameNumber);
            this.blindLevel = asString(payload.blind_level) || this.blindLevel;
            this.boardCards = [];
            this.actionLine = "Hand #" + this.gameNumber + " has started.";
            this.players = this.players.map((player) => ({
              ...player,
              active: false,
              isActing: false,
              isWinner: false,
              cards: [],
              lastAction: "",
            }));
            break;
          case "players_joined":
            this.players = this.players.map((player, index) => ({
              ...player,
              name: Array.isArray(payload.players) && payload.players[index] ? asString(payload.players[index]) : player.name,
              active: true,
              lastAction: "Ready",
            }));
            this.actionLine = "Players are seated and waiting for the deal.";
            break;
          case "card_dealt":
            this.players = this.players.map((player) => {
              if (player.seat !== asNumber(payload.seat, player.seat)) {
                return player;
              }
              return {
                ...player,
                active: true,
                isActing: false,
                cards: normalizeCards(payload.cards),
                lastAction: "Cards dealt",
              };
            });
            this.actionLine = "Pocket cards are on the table.";
            break;
          case "bet_action":
            this.players = this.players.map((player) => {
              const actingSeat = asNumber(payload.seat, 0);
              const action = humanizeToken(asString(payload.action) || "acted");
              const amount = asNumber(payload.amount, 0);
              if (player.seat !== actingSeat) {
                return {
                  ...player,
                  isActing: false,
                };
              }
              return {
                ...player,
                isActing: true,
                active: true,
                lastAction: action + amountSuffix(payload.amount),
              };
            });
            this.triggerBetBurst(asNumber(payload.seat, 0), asNumber(payload.amount, 0));
            this.actionLine =
              "Seat " +
              asNumber(payload.seat, 0) +
              " " +
              humanizeToken(asString(payload.action) || "acted") +
              amountSuffix(payload.amount) +
              ".";
            break;
          case "community_cards":
            this.boardCards = normalizeCards(payload.cards);
            this.players = this.players.map((player) => ({
              ...player,
              isActing: false,
            }));
            this.actionLine = communityStreetLabel(payload.street) + " is on the board.";
            break;
          case "hand_result":
            this.players = this.players.map((player) => ({
              ...player,
              isActing: false,
              isWinner: player.seat === asNumber(payload.winner_seat, -1),
              lastAction: player.seat === asNumber(payload.winner_seat, -1) ? "Winner" : player.lastAction,
            }));
            this.actionLine = asString(payload.summary) || "Hand complete.";
            break;
          default:
            this.actionLine = "Table event received.";
            break;
        }
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
      seatClass(player) {
        let className = "seat-" + player.seat;
        if (player.active) {
          className += " seat-active";
        }
        if (player.isActing) {
          className += " seat-acting";
        }
        if (player.isWinner) {
          className += " seat-winner";
        }
        return className;
      },
      seatBadge(player) {
        if (player.isWinner) {
          return "Winner";
        }
        if (player.isActing) {
          return "Acting";
        }
        return player.lastAction || "Ready";
      },
      boardLabel() {
        if (this.boardCards.length === 0) {
          return "Board waiting";
        }
        return "Community board";
      },
      visibleBoardCards() {
        return this.boardCards;
      },
      boardPlaceholders() {
        return Array.from({ length: Math.max(0, 5 - this.boardCards.length) }, (_, index) => index);
      },
      cardDisplay(card) {
        const text = asString(card);
        if (!text) {
          return "??";
        }

        const rank = text.slice(0, -1) || text;
        const suit = suitSymbol(text.slice(-1));
        return rank + suit;
      },
      cardClass(card) {
        const suit = asString(card).slice(-1).toLowerCase();
        if (suit === "h" || suit === "d") {
          return "playing-card-red";
        }
        return "playing-card-dark";
      },
      playerCards(player) {
        const cards = normalizeCards(player.cards);
        if (cards.length === 0) {
          return ["", ""];
        }
        if (cards.length === 1) {
          return [cards[0], ""];
        }
        return cards.slice(0, 2);
      },
      stackLabel(player) {
        return player.stack + " BB";
      },
      triggerBetBurst(seat, amount) {
        if (!seat || amount <= 0) {
          return;
        }

        const id = "burst_" + seat + "_" + Date.now() + "_" + Math.floor(Math.random() * 1000);
        this.betBursts.push({
          id,
          seat,
          label: amount + " BB",
        });

        window.setTimeout(() => {
          this.betBursts = this.betBursts.filter((burst) => burst.id !== id);
        }, 950);
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

  function defaultPlayers() {
    return [
      { seat: 1, name: "Avery", stack: 100, active: false, isActing: false, isWinner: false, cards: [], lastAction: "" },
      { seat: 2, name: "Blake", stack: 100, active: false, isActing: false, isWinner: false, cards: [], lastAction: "" },
      { seat: 3, name: "Casey", stack: 100, active: false, isActing: false, isWinner: false, cards: [], lastAction: "" },
      { seat: 4, name: "Devon", stack: 100, active: false, isActing: false, isWinner: false, cards: [], lastAction: "" },
    ];
  }

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

  function normalizeCards(cards) {
    if (!Array.isArray(cards)) {
      return [];
    }

    return cards.map((card) => asString(card)).filter(Boolean);
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

  function suitSymbol(suit) {
    switch (asString(suit).toLowerCase()) {
      case "h":
        return "♥";
      case "d":
        return "♦";
      case "c":
        return "♣";
      case "s":
        return "♠";
      default:
        return "";
    }
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
