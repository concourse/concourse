port module BuildPage exposing (..)

import Navigation
import String
import Time
import UrlParser exposing ((</>), s)

import Autoscroll
import Build exposing (Page (..))
import Scroll

port setTitle : String -> Cmd msg

main : Program Never
main =
  Navigation.program
    (Navigation.makeParser pathnameParser)
    { init =
        Autoscroll.init
          Build.getScrollBehavior <<
            Build.init setTitle
    , update = Autoscroll.update Build.update
    , urlUpdate = Autoscroll.urlUpdate Build.urlUpdate
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
                  Sub.map
                    ( Autoscroll.SubAction <<
                        Build.BuildOutputAction model.subModel.browsingIndex
                    )
                    buildOutput.events
            ]
    }

pathnameParser : Navigation.Location -> Result String Page
pathnameParser location =
  UrlParser.parse
    identity
    pageParser
    (String.dropLeft 1 location.pathname)

pageParser : UrlParser.Parser (Page -> a) a
pageParser =
  UrlParser.oneOf
    [ UrlParser.format BuildPage (s "builds" </> UrlParser.int)
    , UrlParser.format
        Build.initJobBuildPage
        ( s "teams" </> UrlParser.string </>
          s "pipelines" </> UrlParser.string </>
          s "jobs" </> UrlParser.string </>
          s "builds" </> UrlParser.string
        )
    ]
