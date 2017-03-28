effect module EventSource
    where { subscription = MySub }
    exposing
        ( listen
        , Msg(..)
        )

import Dict exposing (Dict)
import Process
import Task exposing (Task)
import EventSource.LowLevel as ES


type alias SubscriberKey =
    ( String, List String )


type SelfMsg
    = ESEvent SubscriberKey ES.Event
    | ESOpened SubscriberKey ES.EventSource
    | ESErrored SubscriberKey


type Msg
    = Event ES.Event
    | Opened
    | Errored


listen : SubscriberKey -> (Msg -> msg) -> Sub msg
listen key tagger =
    subscription (MySub key tagger)



-- SUBSCRIPTIONS


type MySub msg
    = MySub SubscriberKey (Msg -> msg)


subMap : (a -> b) -> MySub a -> MySub b
subMap func (MySub key tagger) =
    MySub key (tagger >> func)



-- EFFECT MANAGER


type alias State msg =
    Dict SubscriberKey (Source msg)


type alias Source msg =
    { subs : List (MySub msg)
    , watcher : Process.Id
    , source : Maybe ES.EventSource
    }


init : Task Never (State msg)
init =
    Task.succeed Dict.empty


onEffects : Platform.Router msg SelfMsg -> List (MySub msg) -> State msg -> Task Never (State msg)
onEffects router subs state =
    let
        addSub key tagger msubs =
            Just (MySub key tagger :: Maybe.withDefault [] msubs)

        insertSub (MySub key tagger) state =
            Dict.update key (addSub key tagger) state

        desiredSubs =
            List.foldl insertSub Dict.empty subs
    in
        Dict.merge (createSource router) updateSourceSubs closeSource desiredSubs state (Task.succeed Dict.empty)


createSource : Platform.Router msg SelfMsg -> SubscriberKey -> List (MySub msg) -> Task Never (State msg) -> Task Never (State msg)
createSource router key subs rest =
    rest
        |> Task.andThen
            (\state ->
                Process.spawn (open router key)
                    |> Task.andThen
                        (\processId ->
                            Task.succeed (Dict.insert key { subs = subs, watcher = processId, source = Nothing } state)
                        )
            )


updateSourceSubs : SubscriberKey -> List (MySub msg) -> Source msg -> Task Never (State msg) -> Task Never (State msg)
updateSourceSubs key subs source rest =
    Task.map (Dict.insert key { source | subs = subs }) rest


closeSource : SubscriberKey -> Source msg -> Task Never (State msg) -> Task Never (State msg)
closeSource key source rest =
    rest
        |> Task.andThen
            (\state ->
                case source.source of
                    Nothing ->
                        Task.succeed (Dict.remove key state)

                    Just es ->
                        ES.close es
                            |> Task.andThen
                                (\_ ->
                                    Task.succeed (Dict.remove key state)
                                )
            )


open : Platform.Router msg SelfMsg -> SubscriberKey -> Task Never ES.EventSource
open router ( url, events ) =
    ES.open url
        { events = events
        , onEvent = Platform.sendToSelf router << ESEvent ( url, events )
        , onOpen = Platform.sendToSelf router << ESOpened ( url, events )
        , onError = Platform.sendToSelf router << always (ESErrored ( url, events ))
        }


onSelfMsg : Platform.Router msg SelfMsg -> SelfMsg -> State msg -> Task Never (State msg)
onSelfMsg router msg state =
    case msg of
        ESEvent key ev ->
            case Dict.get key state of
                Nothing ->
                    Task.succeed state

                Just source ->
                    broadcast router (Event ev) source.subs
                        |> Task.andThen
                            (\_ ->
                                Task.succeed state
                            )

        ESOpened key es ->
            case Dict.get key state of
                Nothing ->
                    Task.succeed state

                Just source ->
                    broadcast router Opened source.subs
                        |> Task.andThen
                            (\_ ->
                                Task.succeed (Dict.insert key { source | source = Just es } state)
                            )

        ESErrored key ->
            case Dict.get key state of
                Nothing ->
                    Task.succeed state

                Just source ->
                    broadcast router Errored source.subs
                        |> Task.andThen
                            (\_ ->
                                Task.succeed state
                            )


broadcast : Platform.Router msg SelfMsg -> Msg -> List (MySub msg) -> Task x ()
broadcast router msg subs =
    Task.map (always ()) <|
        Task.sequence <|
            List.map (\(MySub _ tagger) -> Platform.sendToApp router (tagger msg)) subs
