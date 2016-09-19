module Routes exposing (Route(..), parsePath, navigateTo, toString)

import Navigation exposing (Location)
import Route exposing (..)

type Route
  = Login
  | TeamLogin String
  | Pipeline String String

-- homeR =
--   HomeR := static ""

login =
  Login := static "login"

teamLogin =
  TeamLogin := static "teams" </> string </> static "login"

pipeline =
  Pipeline := static "teams" </> string </> static "pipelines" </> string

sitemap =
  router [ login, teamLogin, pipeline ]

match : String -> Route
match =
  Route.match sitemap
      >> Maybe.withDefault Login

toString : Route -> String
toString route =
  case route of
    -- HomeR ->
    --   reverse homeR []

    Login ->
      reverse login []

    TeamLogin teamName ->
      reverse teamLogin [ teamName ]

    Pipeline teamName pipelineName ->
      reverse pipeline [ teamName, pipelineName ]

    -- NotFoundR ->
    --     Debug.crash "cannot render NotFound"

parsePath : Location -> Route
parsePath =
  .pathname >> match

navigateTo : Route -> Cmd msg
navigateTo =
  toString >> Navigation.newUrl
