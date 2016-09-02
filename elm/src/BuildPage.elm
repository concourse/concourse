port module BuildPage exposing (main)

import Navigation
import String
import Time
import UrlParser exposing ((</>), s)

import Autoscroll
import Build exposing (Page (..))

port setTitle : String -> Cmd msg
port focusElement : String -> Cmd msg

main : Program Never
main =
  Navigation.program
    (Navigation.makeParser pathnameParser)
    { init =
        Autoscroll.init
          Build.getScrollBehavior <<
            Build.init
              { setTitle = setTitle
              , focusElement = focusElement
              }
    , update = Autoscroll.update Build.update
    , urlUpdate = Autoscroll.urlUpdate Build.urlUpdate
    , view = Autoscroll.view Build.view
    , subscriptions =
        let
          tick =
            Time.every Time.second (Autoscroll.SubMsg << Build.ClockTick)
        in \model ->
          Sub.batch
            [ tick
            , Autoscroll.subscriptions model
            , Sub.map Autoscroll.SubMsg <|
                Build.subscriptions model.subModel
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
