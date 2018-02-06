module BetaRoutes exposing (ConcourseRoute, Route(..), parsePath, navigateTo, toString, customToString, baseRoute, loginRoute, pipelineRoute, jobRoute, jobIdentifierRoute, buildRoute, teamNameLoginRoute, loginWithRedirectRoute)

import Navigation exposing (Location)
import Route exposing (..)
import QueryString
import Concourse
import Concourse.Pagination as Pagination


type Route
    = BetaHome
    | Dashboard
    | DashboardHd
    | BetaPipeline String String
    | BetaBuild String String String String
    | BetaOneOffBuild String
    | BetaResource String String String
    | BetaJob String String String
    | BetaSelectTeam
    | BetaTeamLogin String


type alias ConcourseRoute =
    { logical : Route
    , queries : QueryString.QueryString
    , page : Maybe Pagination.Page
    , hash : String
    }



-- pages


dashboard : Route.Route Route
dashboard =
    Dashboard := static "beta" </> static "dashboard"


dashboardHd : Route.Route Route
dashboardHd =
    DashboardHd := static "beta" </> static "dashboard" </> static "hd"


betaBuild : Route.Route Route
betaBuild =
    BetaBuild := static "beta" </> static "teams" </> string </> static "pipelines" </> string </> static "jobs" </> string </> static "builds" </> string


betaJob : Route.Route Route
betaJob =
    BetaJob := static "beta" </> static "teams" </> string </> static "pipelines" </> string </> static "jobs" </> string


betaLogin : Route.Route Route
betaLogin =
    BetaSelectTeam := static "beta" </> static "login"


betaPipeline : Route.Route Route
betaPipeline =
    BetaPipeline := static "beta" </> static "teams" </> string </> static "pipelines" </> string


betaOneOffBuild : Route.Route Route
betaOneOffBuild =
    BetaOneOffBuild := static "beta" </> static "builds" </> string


betaResource : Route.Route Route
betaResource =
    BetaResource := static "beta" </> static "teams" </> string </> static "pipelines" </> string </> static "resources" </> string


betaTeamLogin : Route.Route Route
betaTeamLogin =
    BetaTeamLogin := static "beta" </> static "teams" </> string </> static "login"


betaHome : Route.Route Route
betaHome =
    BetaHome := static "beta"



-- route utils


baseRoute : String
baseRoute =
    "/beta"


loginRoute : String
loginRoute =
    baseRoute ++ "/login"


loginWithRedirectRoute : String -> String
loginWithRedirectRoute r =
    baseRoute ++ "/login?redirect=" ++ r


teamNameLoginRoute : String -> String
teamNameLoginRoute teamName =
    (BetaTeamLogin teamName) |> toString


buildRoute : Concourse.Build -> String
buildRoute build =
    case build.job of
        Just j ->
            ((BetaBuild j.teamName j.pipelineName j.jobName build.name) |> toString)

        Nothing ->
            ((BetaOneOffBuild (Basics.toString build.id)) |> toString)


jobRoute : Concourse.Job -> String
jobRoute j =
    ((BetaJob j.teamName j.pipelineName j.name) |> toString)


jobIdentifierRoute : Concourse.JobIdentifier -> String
jobIdentifierRoute j =
    ((BetaJob j.teamName j.pipelineName j.jobName) |> toString)


pipelineRoute : Concourse.Pipeline -> String
pipelineRoute p =
    ((BetaPipeline p.teamName p.name) |> toString)


sitemap : Router Route
sitemap =
    router
        [ dashboard
        , dashboardHd
        , betaPipeline
        , betaBuild
        , betaOneOffBuild
        , betaResource
        , betaJob
        , betaLogin
        , betaTeamLogin
        , betaHome
        ]


match : String -> Route
match =
    Route.match sitemap
        >> Maybe.withDefault BetaHome


toString : Route -> String
toString route =
    case route of
        Dashboard ->
            reverse dashboard []

        DashboardHd ->
            reverse dashboardHd []

        BetaPipeline teamName pipelineName ->
            reverse betaPipeline [ teamName, pipelineName ]

        BetaBuild teamName pipelineName jobName buildName ->
            reverse betaBuild [ teamName, pipelineName, jobName, buildName ]

        BetaOneOffBuild buildId ->
            reverse betaOneOffBuild [ buildId ]

        BetaResource teamName pipelineName resourceName ->
            reverse betaJob [ teamName, pipelineName, resourceName ]

        BetaJob teamName pipelineName jobName ->
            reverse betaJob [ teamName, pipelineName, jobName ]

        BetaSelectTeam ->
            reverse betaLogin []

        BetaTeamLogin teamName ->
            reverse betaTeamLogin [ teamName ]

        BetaHome ->
            "/beta"


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
