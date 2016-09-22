module Routes exposing (ConcourseRoute, Route(..), parsePath, navigateTo, toString)

import Erl
import Navigation exposing (Location)
import Route exposing (..)

type Route
  = SelectTeam
  | TeamLogin String
  | Pipeline String String

type alias ConcourseRoute =
  { logical : Route
  , parsed : Erl.Url
  }

login : Route.Route Route
login =
  SelectTeam := static "login"

teamLogin : Route.Route Route
teamLogin =
  TeamLogin := static "teams" </> string </> static "login"

pipeline : Route.Route Route
pipeline =
  Pipeline := static "teams" </> string </> static "pipelines" </> string

sitemap : Router Route
sitemap =
  router [ login, teamLogin, pipeline ]

match : String -> Route
match =
  Route.match sitemap
      >> Maybe.withDefault SelectTeam

toString : Route -> String
toString route =
  case route of

    SelectTeam ->
      reverse login []

    TeamLogin teamName ->
      reverse teamLogin [ teamName ]

    Pipeline teamName pipelineName ->
      reverse pipeline [ teamName, pipelineName ]

parsePath : Location -> ConcourseRoute
parsePath location =
  let
    parsed =
      Erl.parse location.href
  in
    { logical = match <| location.pathname
    , parsed = parsed
    }

navigateTo : Route -> Cmd msg
navigateTo =
  toString >> Navigation.newUrl
