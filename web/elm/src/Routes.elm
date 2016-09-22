module Routes exposing (Route(..), parsePath, navigateTo, toString)

import Erl
import Navigation exposing (Location)
import Route exposing (..)

type Route
  = Login
  | TeamLogin String
  | Pipeline String String

type alias ConcourseRoute
  { route : Route
  , params : Erl.Query
  }

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

    Login ->
      reverse login []

    TeamLogin teamName ->
      reverse teamLogin [ teamName ]

    Pipeline teamName pipelineName ->
      reverse pipeline [ teamName, pipelineName ]

parsePath : Location -> ConcourseRoute
parsePath location =
  let
    parsed =
      Erl.parse location.url
  in
  { route = match <| location.pathname
  , query = parsed.query
  }

navigateTo : Route -> Cmd msg
navigateTo =
  toString >> Navigation.newUrl
