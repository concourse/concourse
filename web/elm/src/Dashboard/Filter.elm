module Dashboard.Filter exposing (Suggestion, filterTeams, suggestions)

import Concourse exposing (DatabaseID, flattenJson)
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        , equal
        , isRunning
        )
import Dashboard.Group.Models exposing (Card(..), Pipeline)
import Dashboard.Pipeline as Pipeline
import Dict exposing (Dict)
import FetchResult exposing (FetchResult)
import List.Extra
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , Step(..)
        , backtrackable
        , chompUntilEndOr
        , chompWhile
        , end
        , getChompedString
        , getOffset
        , getSource
        , keyword
        , loop
        , map
        , oneOf
        , run
        , spaces
        , succeed
        , symbol
        )
import Routes
import Set exposing (Set)
import Simple.Fuzzy


type alias Filter =
    { negate : Bool
    , groupFilter : GroupFilter
    }


type GroupFilter
    = Team StringFilter
    | Pipeline PipelineFilter


type PipelineFilter
    = Name StringFilter
    | Status StatusFilter


type StringFilter
    = Fuzzy String
    | Exact String
    | StartsWith String


type StatusFilter
    = PipelineStatus PipelineStatus
    | PipelineRunning
    | IncompleteStatus String


filterTypes : List String
filterTypes =
    [ "status", "team" ]


filterTeams :
    { pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobName)
    , jobs : Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    , query : String
    , teams : List Concourse.Team
    , pipelines : Dict String (List Pipeline)
    , dashboardView : Routes.DashboardView
    , favoritedPipelines : Set DatabaseID
    }
    -> Dict String (List Pipeline)
filterTeams { pipelineJobs, jobs, query, teams, pipelines, dashboardView, favoritedPipelines } =
    let
        teamsToFilter =
            teams
                |> List.map (\t -> ( t.name, [] ))
                |> Dict.fromList
                |> Dict.union pipelines
                |> Dict.map
                    (\_ p ->
                        List.filter (prefilter dashboardView favoritedPipelines) p
                    )
    in
    parseFilters query |> List.map Tuple.first |> List.foldr (runFilter jobs pipelineJobs) teamsToFilter


prefilter : Routes.DashboardView -> Set DatabaseID -> Pipeline -> Bool
prefilter view favoritedPipelines p =
    case view of
        Routes.ViewNonArchivedPipelines ->
            not p.archived || Set.member p.id favoritedPipelines

        _ ->
            True


runFilter :
    Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    -> Dict Concourse.DatabaseID (List Concourse.JobName)
    -> Filter
    -> Dict String (List Pipeline)
    -> Dict String (List Pipeline)
runFilter jobs existingJobs f =
    let
        negater =
            if f.negate then
                not

            else
                identity
    in
    case f.groupFilter of
        Team sf ->
            Dict.filter (\team _ -> stringMatches sf team |> negater)

        Pipeline pf ->
            Dict.map
                (\_ pipelines -> List.filter (pipelineFilter pf jobs existingJobs >> negater) pipelines)
                >> Dict.filter (\_ pipelines -> not <| List.isEmpty pipelines)


pipelineFilter :
    PipelineFilter
    -> Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    -> Dict Concourse.DatabaseID (List Concourse.JobName)
    -> Pipeline
    -> Bool
pipelineFilter pf jobs existingJobs pipeline =
    let
        jobsForPipeline =
            existingJobs
                |> Dict.get pipeline.id
                |> Maybe.withDefault []
                |> List.filterMap (\j -> Dict.get ( pipeline.id, j ) jobs)
    in
    case pf of
        Name sf ->
            let
                instanceVarValues =
                    pipeline.instanceVars
                        |> Dict.toList
                        |> List.concatMap (\( k, v ) -> flattenJson k v)
                        |> List.map Tuple.second
            in
            List.any (stringMatches sf) (pipeline.name :: instanceVarValues)

        Status sf ->
            case sf of
                PipelineStatus ps ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> equal ps

                PipelineRunning ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> isRunning

                IncompleteStatus _ ->
                    False


stringMatches : StringFilter -> String -> Bool
stringMatches f =
    case f of
        Fuzzy term ->
            Simple.Fuzzy.match term

        Exact name ->
            (==) name

        StartsWith prefix ->
            String.startsWith prefix


parseFilters : String -> List ( Filter, String )
parseFilters =
    run
        (loop [] <|
            \revFilters ->
                oneOf
                    [ end |> map (\_ -> Done (List.reverse revFilters))
                    , filter |> captureChompedString |> map (\f -> Loop (f :: revFilters))
                    ]
        )
        >> Result.withDefault []


captureChompedString : Parser a -> Parser ( a, String )
captureChompedString parser =
    succeed (\start val end source -> ( val, String.slice start end source ))
        |= getOffset
        |= parser
        |= getOffset
        |= getSource


filter : Parser Filter
filter =
    succeed Filter
        |. spaces
        |= oneOf
            [ symbol "-" |> map (always True)
            , succeed False
            ]
        |= groupFilter


groupFilter : Parser GroupFilter
groupFilter =
    oneOf
        [ backtrackable teamFilter
        , backtrackable statusFilter
        , succeed (Name >> Pipeline) |= parseString
        ]


parseString : Parser StringFilter
parseString =
    oneOf
        [ parseQuotedString
        , parseWord |> map Fuzzy
        ]


parseQuotedString : Parser StringFilter
parseQuotedString =
    getChompedString
        (symbol "\""
            |. chompUntilEndOr "\""
            |. oneOf [ symbol "\"", end ]
        )
        |> map
            (\s ->
                if String.endsWith "\"" s && s /= "\"" then
                    Exact <| String.slice 1 -1 s

                else
                    StartsWith <| String.dropLeft 1 s
            )


parseWord : Parser String
parseWord =
    getChompedString
        (chompWhile
            (\c -> c /= ' ' && c /= '\t' && c /= '\n' && c /= '\u{000D}')
        )


teamFilter : Parser GroupFilter
teamFilter =
    succeed Team
        |. keyword "team"
        |. symbol ":"
        |. spaces
        |= parseString


statusFilter : Parser GroupFilter
statusFilter =
    succeed (Status >> Pipeline)
        |. keyword "status"
        |. symbol ":"
        |. spaces
        |= pipelineStatus


pipelineStatus : Parser StatusFilter
pipelineStatus =
    oneOf
        [ keyword "paused" |> map (\_ -> PipelineStatus PipelineStatusPaused)
        , keyword "aborted" |> map (\_ -> PipelineStatus <| PipelineStatusAborted Running)
        , keyword "errored" |> map (\_ -> PipelineStatus <| PipelineStatusErrored Running)
        , keyword "failed" |> map (\_ -> PipelineStatus <| PipelineStatusFailed Running)
        , keyword "pending" |> map (\_ -> PipelineStatus <| PipelineStatusPending False)
        , keyword "succeeded" |> map (\_ -> PipelineStatus <| PipelineStatusSucceeded Running)
        , keyword "running" |> map (\_ -> PipelineRunning)
        , parseWord |> map IncompleteStatus
        ]


type alias Suggestion =
    { prev : String
    , cur : String
    }


suggestions :
    { a
        | query : String
        , teams : FetchResult (List Concourse.Team)
        , pipelines : Maybe (Dict String (List Pipeline))
    }
    -> List Suggestion
suggestions { query, teams, pipelines } =
    let
        parsedFilters =
            parseFilters query

        ( curFilter, negated ) =
            parsedFilters
                |> List.Extra.last
                |> Maybe.map Tuple.first
                |> Maybe.map (\f -> ( f.groupFilter, f.negate ))
                |> Maybe.withDefault ( Pipeline (Name (Fuzzy "")), False )

        prevFilters =
            parsedFilters
                |> List.map Tuple.second
                |> List.reverse
                |> List.drop 1
                |> List.reverse

        prev =
            if List.isEmpty prevFilters then
                ""

            else
                String.join "" prevFilters ++ " "

        cur =
            case curFilter of
                Pipeline (Name (Fuzzy s)) ->
                    filterTypes
                        |> List.filter (String.startsWith s)
                        |> List.map (\v -> v ++ ":")

                Pipeline (Name _) ->
                    []

                Pipeline (Status sf) ->
                    case sf of
                        IncompleteStatus status ->
                            [ "paused", "pending", "failed", "errored", "aborted", "running", "succeeded" ]
                                |> List.filter (String.startsWith status)
                                |> List.map (\v -> "status:" ++ v)

                        _ ->
                            []

                Team (Exact _) ->
                    []

                Team team ->
                    Set.union
                        (teams
                            |> FetchResult.withDefault []
                            |> List.map .name
                            |> Set.fromList
                        )
                        (pipelines
                            |> Maybe.withDefault Dict.empty
                            |> Dict.keys
                            |> Set.fromList
                        )
                        |> Set.toList
                        |> List.filter (stringMatches team)
                        |> List.take 10
                        |> List.map (\v -> "team:" ++ quoted v)

        prefix =
            if negated then
                List.map (\c -> "-" ++ c)

            else
                identity
    in
    List.map (Suggestion prev) (prefix cur)


quoted : String -> String
quoted s =
    "\"" ++ s ++ "\""
