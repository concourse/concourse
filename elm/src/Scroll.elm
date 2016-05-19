effect module Scroll where { subscription = MySub } exposing
  ( toBottom
  , fromBottom
  , scroll
  , scrollIntoView
  )

import Dom.LowLevel as Dom
import Json.Decode as Json exposing ((:=))
import Process
import Task exposing (Task)

import Native.Scroll


type alias FromBottom =
  Int

decodeFromBottom : Json.Decoder FromBottom
decodeFromBottom =
  Json.customDecoder decodeComparators <| \(yOffset, clientHeight, scrollHeight) ->
    let
      scrolledHeight = yOffset - clientHeight
    in
      Ok (scrollHeight - scrolledHeight)

decodeComparators : Json.Decoder (Int, Int, Int)
decodeComparators =
  Json.object3 (,,)
    (Json.at ["currentTarget", "pageYOffset"] Json.int)
    (Json.at ["target", "documentElement", "clientHeight"] Json.int)
    (Json.at ["target", "documentElement", "scrollHeight"] Json.int)


-- SCROLL EVENTS


fromBottom : (FromBottom -> msg) -> Sub msg
fromBottom tagger =
  subscription (MySub tagger)

toBottom : Task x ()
toBottom =
  Native.Scroll.toBottom ()

scroll : String -> Float -> Task x ()
scroll =
  Native.Scroll.scroll

scrollIntoView : String -> Task x ()
scrollIntoView =
  Native.Scroll.scrollIntoView


-- SUBSCRIPTIONS


type MySub msg =
  MySub (FromBottom -> msg)


subMap : (a -> b) -> MySub a -> MySub b
subMap func (MySub tagger) =
  MySub (tagger >> func)



-- EFFECT MANAGER


type alias State msg =
  Maybe
    { subs : List (MySub msg)
    , watcher : Process.Id
    }


init : Task Never (State msg)
init =
  Task.succeed Nothing


onEffects : Platform.Router msg FromBottom -> List (MySub msg) -> State msg -> Task Never (State msg)
onEffects router subs state =
  case state of
    Nothing ->
      case subs of
        [] ->
          Task.succeed state

        _ ->
          Process.spawn (Dom.onWindow "scroll" decodeFromBottom (Platform.sendToSelf router))
            `Task.andThen` \watcher ->

          Task.succeed (Just { subs = subs, watcher = watcher })

    Just {subs,watcher} ->
      case subs of
        [] ->
          Process.kill watcher
            `Task.andThen` \_ ->

          Task.succeed Nothing

        _ ->
          Task.succeed (Just { subs = subs, watcher = watcher })


onSelfMsg : Platform.Router msg FromBottom -> FromBottom -> State msg -> Task Never (State msg)
onSelfMsg router fb state =
  case state of
    Nothing ->
      Task.succeed Nothing

    Just {subs} ->
      let
        send (MySub tagger) =
          Platform.sendToApp router (tagger fb)
      in
        Task.sequence (List.map send subs)
          `Task.andThen` \_ ->

        Task.succeed state
