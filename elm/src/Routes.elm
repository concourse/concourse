module Routes exposing (ConcourseRoute, Route(..), customToString, navigateTo, parsePath, pipelineRoute, jobRoute, buildRoute, dashboardRoute, dashboardHdRoute, toString)

import Concourse
import Concourse.Pagination as Pagination
import Navigation exposing (Location)
import QueryString
import Route exposing (..)


type Route
    = Build String String String String
    | Resource String String String
    | Job String String String
    | OneOffBuild String
    | Pipeline String String
    | Dashboard
    | DashboardHd


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


job : Route.Route Route
job =
    Job := static "teams" </> string </> static "pipelines" </> string </> static "jobs" </> string


pipeline : Route.Route Route
pipeline =
    Pipeline := static "teams" </> string </> static "pipelines" </> string


dashboard : Route.Route Route
dashboard =
    Dashboard := static ""


dashboardHd : Route.Route Route
dashboardHd =
    DashboardHd := static "hd"



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


dashboardRoute : String
dashboardRoute =
    Dashboard |> toString


dashboardHdRoute : String
dashboardHdRoute =
    DashboardHd |> toString



-- router


sitemap : Router Route
sitemap =
    router
        [ build
        , resource
        , job
        , oneOffBuild
        , pipeline
        , dashboard
        , dashboardHd
        ]


match : String -> Route
match =
    Route.match sitemap
        >> Maybe.withDefault Dashboard


toString : Route -> String
toString route =
    case route of
        Build teamName pipelineName jobName buildName ->
            reverse build [ teamName, pipelineName, jobName, buildName ]

        Job teamName pipelineName jobName ->
            reverse job [ teamName, pipelineName, jobName ]

        Resource teamName pipelineName resourceName ->
            reverse resource [ teamName, pipelineName, resourceName ]

        OneOffBuild buildId ->
            reverse oneOffBuild [ buildId ]

        Pipeline teamName pipelineName ->
            reverse pipeline [ teamName, pipelineName ]

        Dashboard ->
            reverse dashboard []

        DashboardHd ->
            reverse dashboardHd []


parsePath : Location -> ConcourseRoute
parsePath location =
    { logical = match <| location.pathname
    , queries =
        QueryString.parse location.search
            |> QueryString.remove "csrf_token"
            |> QueryString.remove "token"
    , page = createPageFromSearch location.search
    , hash = location.hash
    }


customToString : ConcourseRoute -> String
customToString route =
    toString route.logical
        ++ if route.queries == QueryString.empty then
            ""
           else
            QueryString.render route.queries


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
