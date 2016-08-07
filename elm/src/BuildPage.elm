port module BuildPage exposing (..)

import Html.App
import Time

import Autoscroll
import Build
import Scroll

main : Program Build.Flags
main =
  Html.App.programWithFlags
    { init =
        Autoscroll.init
          Build.getScrollBehavior <<
            Build.init
    , update = Autoscroll.update Build.update
    , view = Autoscroll.view Build.view
    , subscriptions =
        let
          tick =
            Time.every Time.second (Autoscroll.SubAction << Build.ClockTick)

          scrolledUp =
            Scroll.fromBottom Autoscroll.FromBottom

          pushDown =
            Time.every (100 * Time.millisecond) (always Autoscroll.ScrollDown)
        in \model ->
          Sub.batch
            [ tick
            , scrolledUp
            , pushDown
            , case model.subModel.currentBuild `Maybe.andThen` Build.currentBuildOutput of
                Nothing ->
                  Sub.none
                Just buildOutput ->
                  Sub.map (Autoscroll.SubAction << Build.BuildOutputAction) buildOutput.events
            ]
    }
