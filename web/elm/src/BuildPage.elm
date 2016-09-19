port module BuildPage exposing (main)

import Navigation
import String
import UrlParser exposing ((</>), s)

import Autoscroll
import Build exposing (Page (..))

port setTitle : String -> Cmd msg
port focusElement : String -> Cmd msg
port selectBuildGroups : List String -> Cmd msg

main : Program Never
main =
  { init =
      Autoscroll.init
        Build.getScrollBehavior <<
          Build.init
            { setTitle = setTitle
            , focusElement = focusElement
            , selectGroups = selectBuildGroups
            }
  , update = Autoscroll.update Build.update
  , view = Autoscroll.view Build.view
  , subscriptions = Autoscroll.subscriptions Build.subscriptions
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
