module StartApp ( start, Config, App ) where
{-| This module helps you start your application in a typical Elm workflow.
It assumes you are following [the Elm Architecture][arch] and using
[elm-effects][]. From there it will wire everything up for you!
**Be sure to [read the Elm Architecture tutorial][arch] to learn how this all
works!**
[arch]: https://github.com/evancz/elm-architecture-tutorial
[elm-effects]: http://package.elm-lang.org/packages/evancz/elm-effects/latest
# Start your Application
@docs start, Config, App
-}

import Html exposing (Html)
import Task
import Effects exposing (Effects, Never)
import Signal.Extra exposing (foldp', mapMany)


{-| The configuration of an app follows the basic model / update / view pattern
that you see in every Elm program.
The `init` transaction will give you an initial model and create any tasks that
are needed on start up.
The `update` and `view` fields describe how to step the model and view the
model.
The `inputs` field is for any external signals you might need. If you need to
get values from JavaScript, they will come in through a port as a signal which
you can pipe into your app as one of the `inputs`.
The `inits` field works similarly to `inputs`, but the initial values of each signal
will be applied when the application loads.
-}
type alias Config model action =
    { init : (model, Effects action)
    , update : action -> model -> (model, Effects action)
    , view : Signal.Address action -> model -> Html
    , inputs : List (Signal.Signal action)
    , inits : List (Signal.Signal action)
    }


{-| An `App` is made up of a couple signals:
  * `html` &mdash; a signal of `Html` representing the current visual
    representation of your app. This should be fed into `main`.
  * `model` &mdash; a signal representing the current model. Generally you
    will not need this one, but it is there just in case. You will know if you
    need this.
  * `tasks` &mdash; a signal of tasks that need to get run. Your app is going
    to be producing tasks in response to all sorts of events, so this needs to
    be hooked up to a `port` to ensure they get run.
-}
type alias App model =
    { html : Signal Html
    , model : Signal model
    , tasks : Signal (Task.Task Never ())
    }


{-| Start an application. It requires a bit of wiring once you have created an
`App`. It should pretty much always look like this:
    app =
        start { init = init, view = view, update = update, inputs = [] }
    main =
        app.html
    port tasks : Signal (Task.Task Never ())
    port tasks =
        app.tasks
So once we start the `App` we feed the HTML into `main` and feed the resulting
tasks into a `port` that will run them all.
-}
start : Config model action -> App model
start config =
    let
        singleton action = [ action ]

        -- messages : Signal.Mailbox (List action)
        messages =
            Signal.mailbox []

        -- address : Signal.Address action
        address =
            Signal.forwardTo messages.address singleton

        -- updateStep : (Bool, action) -> (model, Effects action) -> (model, Effects action)
        updateStep (_, action) (oldModel, accumulatedEffects) =
            let
                (newModel, additionalEffects) = config.update action oldModel
            in
                (newModel, Effects.batch [accumulatedEffects, additionalEffects])

        -- update : List (Bool, action) -> (model, Effects action) -> (model, Effects action)
        update actions (model, _) =
            List.foldl updateStep (model, Effects.none) actions

        -- updateStart : List (Bool, action) -> (model, Effects action)
        updateStart actions =
            List.foldl updateStep config.init (List.filter fst actions)

        -- inputs : Signal (List (Bool, action))
        inputs =
          List.foldl
            (Signal.Extra.fairMerge List.append)
            (Signal.map (List.map ((,) False)) messages.signal)
            (List.map (Signal.map (singleton << (,) False)) config.inputs ++ List.map (Signal.map (singleton << (,) True)) config.inits)

        -- effectsAndModel : Signal (model, Effects action)
        effectsAndModel =
            foldp' update updateStart inputs

        model =
            Signal.map fst effectsAndModel
    in
        { html = Signal.map (config.view address) model
        , model = model
        , tasks = Signal.map (Effects.toTask messages.address << snd) effectsAndModel
        }
