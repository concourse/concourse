module Pinned exposing (..)


type ResourcePinState version id
    = NotPinned
    | PinningTo id
    | PinnedDynamicallyTo version
    | UnpinningFrom version
    | PinnedStaticallyTo version


type VersionPinState
    = Enabled
    | PinnedDynamically
    | PinnedStatically { showTooltip : Bool }
    | Disabled
    | InTransition


startPinningTo : id -> ResourcePinState version id -> ResourcePinState version id
startPinningTo destination resourcePinState =
    case resourcePinState of
        NotPinned ->
            PinningTo destination

        x ->
            x


finishPinning : (id -> Maybe version) -> ResourcePinState version id -> ResourcePinState version id
finishPinning lookup resourcePinState =
    case resourcePinState of
        PinningTo b ->
            lookup b |> Maybe.map PinnedDynamicallyTo |> Maybe.withDefault NotPinned

        x ->
            x


startUnpinning : ResourcePinState version id -> ResourcePinState version id
startUnpinning resourcePinState =
    case resourcePinState of
        PinnedDynamicallyTo v ->
            UnpinningFrom v

        x ->
            x


quitUnpinning : ResourcePinState version id -> ResourcePinState version id
quitUnpinning resourcePinState =
    case resourcePinState of
        UnpinningFrom v ->
            PinnedDynamicallyTo v

        x ->
            x


stable : ResourcePinState version id -> Maybe version
stable version =
    case version of
        PinnedStaticallyTo v ->
            Just v

        PinnedDynamicallyTo v ->
            Just v

        _ ->
            Nothing


pinState : version -> id -> ResourcePinState version id -> VersionPinState
pinState version id resourcePinState =
    case resourcePinState of
        PinnedStaticallyTo v ->
            if v == version then
                PinnedStatically { showTooltip = False }
            else
                Disabled

        NotPinned ->
            Enabled

        PinningTo destination ->
            if destination == id then
                InTransition
            else
                Disabled

        PinnedDynamicallyTo v ->
            if v == version then
                PinnedDynamically
            else
                Disabled

        UnpinningFrom v ->
            if v == version then
                InTransition
            else
                Disabled
