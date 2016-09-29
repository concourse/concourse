module Routes exposing (ConcourseRoute, Route(..), parsePath, navigateTo, toString)

import Erl
import Navigation exposing (Location)
import Route exposing (..)

type Route
  = Home
  | Build String String String String
  | Resource String String String
  | Job String String String
  | OneOffBuild String
  | Pipeline String String
  | SelectTeam
  | TeamLogin String

type alias ConcourseRoute =
  { logical : Route
  , parsed : Erl.Url
  }

-- pages

build : Route.Route Route
build =
  Build := static "teams" </> string </> static "pipelines" </> string </> static "jobs" </> string </> static "builds" </> string

oneOffBuild : Route.Route Route
oneOffBuild =
  OneOffBuild := static "builds" </> string

resource : Route.Route Route
resource =
  Resource := static "teams" </> string </> static "pipelines" </> string </> static "resources" </> string

job : Route.Route Route
job =
  Job := static "teams" </> string </> static "pipelines" </> string </> static "jobs" </> string

login : Route.Route Route
login =
  SelectTeam := static "login"

pipeline : Route.Route Route
pipeline =
  Pipeline := static "teams" </> string </> static "pipelines" </> string

teamLogin : Route.Route Route
teamLogin =
  TeamLogin := static "teams" </> string </> static "login"

-- router

sitemap : Router Route
sitemap =
  router
    [ build
    , resource
    , job
    , login
    , oneOffBuild
    , pipeline
    , teamLogin
    ]

match : String -> Route
match =
  Route.match sitemap
      >> Maybe.withDefault Home

toString : Route -> String
toString route =
  case route of
    Build teamName pipelineName jobName buildName ->
      reverse build [ teamName, pipelineName, jobName, buildName ]
    Job teamName pipelineName jobName ->
      reverse job [ teamName, pipelineName, jobName ]
    Resource teamName pipelineName resourceName ->
      reverse job [ teamName, pipelineName, resourceName ]
    OneOffBuild buildId ->
      reverse oneOffBuild [ buildId ]
    Pipeline teamName pipelineName ->
      reverse pipeline [ teamName, pipelineName ]
    SelectTeam ->
      reverse login []
    TeamLogin teamName ->
      reverse teamLogin [ teamName ]
    Home ->
      "/"

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
