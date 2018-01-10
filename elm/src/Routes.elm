module Routes exposing (ConcourseRoute, Route(..), customToString, navigateTo, parsePath, pipelineRoute, jobRoute, buildRoute, toString)

import Concourse
import Concourse.Pagination as Pagination
import Navigation exposing (Location)
import QueryString
import Route exposing (..)


type Route
    = Home
    | Build String String String String
    | Resource String String String
    | BetaResource String String String
    | Job String String String
    | OneOffBuild String
    | Pipeline String String
    | SelectTeam
    | TeamLogin String


type alias ConcourseRoute =
    { logical : Route
    , queries : QueryString.QueryString
    , page : Maybe Pagination.Page
    , hash : String
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


betaResource : Route.Route Route
betaResource =
    BetaResource := static "beta" </> static "teams" </> string </> static "pipelines" </> string </> static "resources" </> string


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



-- route utils


buildRoute : Concourse.Build -> String
buildRoute build =
    case build.job of
        Just j ->
            (Build j.teamName j.pipelineName j.jobName build.name) |> toString

        Nothing ->
            (OneOffBuild (Basics.toString build.id)) |> toString


jobRoute : Concourse.Job -> String
jobRoute j =
    (Job j.teamName j.pipelineName j.name) |> toString


pipelineRoute : Concourse.Pipeline -> String
pipelineRoute p =
    (Pipeline p.teamName p.name) |> toString



-- router


sitemap : Router Route
sitemap =
    router
        [ build
        , resource
        , betaResource
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

        BetaResource teamName pipelineName resourceName ->
            reverse betaResource [ teamName, pipelineName, resourceName ]

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
    { logical = match <| location.pathname
    , queries = QueryString.parse location.search |> QueryString.remove "csrf_token"
    , page = createPageFromSearch location.search
    , hash = location.hash
    }


customToString : ConcourseRoute -> String
customToString route =
    toString route.logical ++ QueryString.render route.queries


createPageFromSearch : String -> Maybe Pagination.Page
createPageFromSearch search =
    let
        q =
            QueryString.parse search

        until =
            QueryString.one QueryString.int "until" q

        since =
            QueryString.one QueryString.int "since" q

        limit =
            Maybe.withDefault 100 <| QueryString.one QueryString.int "limit" q
    in
        case ( since, until ) of
            ( Nothing, Just u ) ->
                Just
                    { direction = Pagination.Until u
                    , limit = limit
                    }

            ( Just s, Nothing ) ->
                Just
                    { direction = Pagination.Since s
                    , limit = limit
                    }

            _ ->
                Nothing


navigateTo : Route -> Cmd msg
navigateTo =
    toString >> Navigation.newUrl
