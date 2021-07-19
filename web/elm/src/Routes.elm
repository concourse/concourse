module Routes exposing
    ( DashboardView(..)
    , Highlight(..)
    , Route(..)
    , SearchType(..)
    , StepID
    , Transition
    , buildRoute
    , extractPid
    , extractQuery
    , getGroups
    , jobRoute
    , parsePath
    , pipelineRoute
    , resourceRoute
    , searchQueryParams
    , showHighlight
    , toString
    , tokenToFlyRoute
    , versionQueryParams
    , withGroups
    )

import Api.Pagination
import Concourse exposing (InstanceVars, JsonValue(..))
import Concourse.Pagination as Pagination exposing (Direction(..))
import Dict exposing (Dict)
import DotNotation
import Maybe.Extra
import RouteBuilder exposing (RouteBuilder, appendPath, appendQuery)
import Url
import Url.Builder as Builder
import Url.Parser
    exposing
        ( (</>)
        , (<?>)
        , Parser
        , custom
        , fragment
        , int
        , map
        , oneOf
        , parse
        , s
        , string
        , top
        )
import Url.Parser.Query as Query


type Route
    = Build { id : Concourse.JobBuildIdentifier, highlight : Highlight, groups : List String }
    | Resource { id : Concourse.ResourceIdentifier, page : Maybe Pagination.Page, version : Maybe Concourse.Version, groups : List String }
    | Job { id : Concourse.JobIdentifier, page : Maybe Pagination.Page, groups : List String }
    | OneOffBuild { id : Concourse.BuildId, highlight : Highlight }
    | Pipeline { id : Concourse.PipelineIdentifier, groups : List String }
    | Dashboard { searchType : SearchType, dashboardView : DashboardView }
    | FlySuccess Bool (Maybe Int)
      -- the version field is really only used as a hack to populate the breadcrumbs, it's not actually used by anyhting else
    | Causality { id : Concourse.VersionedResourceIdentifier, direction : Concourse.CausalityDirection, version : Maybe Concourse.Version, groups : List String }


type SearchType
    = HighDensity
    | Normal String


type DashboardView
    = ViewNonArchivedPipelines
    | ViewAllPipelines


dashboardViews : List DashboardView
dashboardViews =
    [ ViewNonArchivedPipelines, ViewAllPipelines ]


dashboardViewName : DashboardView -> String
dashboardViewName view =
    case view of
        ViewAllPipelines ->
            "all"

        ViewNonArchivedPipelines ->
            "non_archived"


type Highlight
    = HighlightNothing
    | HighlightLine StepID Int
    | HighlightRange StepID Int Int


type alias StepID =
    String


type alias Transition =
    { from : Route
    , to : Route
    }



-- pages


pipelineIdentifier : Parser ({ teamName : String, pipelineName : String } -> a) a
pipelineIdentifier =
    s "teams"
        </> string
        </> s "pipelines"
        </> string
        |> map
            (\t p ->
                { teamName = t
                , pipelineName = p
                }
            )


build : Parser ((InstanceVars -> Route) -> a) a
build =
    let
        buildHelper { teamName, pipelineName } jobName buildName h =
            \iv ->
                Build
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , jobName = jobName
                        , buildName = buildName
                        }
                    , highlight = h
                    , groups = []
                    }
    in
    map buildHelper
        (pipelineIdentifier
            </> s "jobs"
            </> string
            </> s "builds"
            </> string
            </> fragment parseHighlight
        )


oneOffBuild : Parser ((b -> Route) -> a) a
oneOffBuild =
    map
        (\b h -> always <| OneOffBuild { id = b, highlight = h })
        (s "builds" </> int </> fragment parseHighlight)


parsePage : Maybe Int -> Maybe Int -> Maybe Int -> Maybe Pagination.Page
parsePage from to limit =
    case ( from, to, limit ) of
        ( Nothing, Just t, Just l ) ->
            Just
                { direction = Pagination.To t
                , limit = l
                }

        ( Just f, Nothing, Just l ) ->
            Just
                { direction = Pagination.From f
                , limit = l
                }

        _ ->
            Nothing


resource : Parser ((InstanceVars -> Route) -> a) a
resource =
    let
        resourceHelper { teamName, pipelineName } resourceName from to limit version =
            \iv ->
                Resource
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , resourceName = resourceName
                        }
                    , page = parsePage from to limit
                    , version = version
                    , groups = []
                    }
    in
    map resourceHelper
        (pipelineIdentifier
            </> s "resources"
            </> string
            <?> Query.int "from"
            <?> Query.int "to"
            <?> Query.int "limit"
            <?> resourceVersion "filter"
        )


resourceVersion : String -> Query.Parser (Maybe Concourse.Version)
resourceVersion key =
    let
        split s =
            case String.split ":" s of
                x :: xs ->
                    Just ( x, String.join ":" xs )

                _ ->
                    Nothing

        parse queries =
            List.map split queries |> Maybe.Extra.values |> Dict.fromList

        clean queries =
            if Dict.isEmpty <| parse queries then
                Nothing

            else
                Just <| parse queries
    in
    Query.custom key clean


job : Parser ((InstanceVars -> Route) -> a) a
job =
    let
        jobHelper { teamName, pipelineName } jobName from to limit =
            \iv ->
                Job
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , jobName = jobName
                        }
                    , page = parsePage from to limit
                    , groups = []
                    }
    in
    map jobHelper
        (pipelineIdentifier
            </> s "jobs"
            </> string
            <?> Query.int "from"
            <?> Query.int "to"
            <?> Query.int "limit"
        )


pipeline : Parser ((InstanceVars -> Route) -> a) a
pipeline =
    map
        (\{ teamName, pipelineName } g ->
            \iv ->
                Pipeline
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        }
                    , groups = g
                    }
        )
        (pipelineIdentifier <?> Query.custom "group" identity)


dashboard : Parser ((b -> Route) -> a) a
dashboard =
    map (\st view -> always <| Dashboard { searchType = st, dashboardView = view }) <|
        oneOf
            [ (top
                <?> (stringWithSpaces "search" |> Query.map (Maybe.withDefault ""))
              )
                |> map Normal
            , s "hd" |> map HighDensity
            ]
            <?> dashboardViewQuery


dashboardViewQuery : Query.Parser DashboardView
dashboardViewQuery =
    (Query.enum "view" <|
        Dict.fromList
            (dashboardViews
                |> List.map (\v -> ( dashboardViewName v, v ))
            )
    )
        |> Query.map (Maybe.withDefault ViewNonArchivedPipelines)


stringWithSpaces : String -> Query.Parser (Maybe String)
stringWithSpaces =
    -- https://github.com/elm/url/issues/32
    Query.string >> Query.map (Maybe.map (String.replace "+" " "))


flySuccess : Parser ((b -> Route) -> a) a
flySuccess =
    map (\s p -> always <| FlySuccess (s == Just "true") p)
        (s "fly_success"
            <?> Query.string "noop"
            <?> Query.int "fly_port"
        )


causality : Parser ((InstanceVars -> Route) -> a) a
causality =
    let
        causalityHelper direction { teamName, pipelineName } resourceName versionId =
            \iv ->
                Causality
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , resourceName = resourceName
                        , versionID = versionId
                        }
                    , direction = direction
                    , version = Nothing
                    , groups = []
                    }

        baseRoute dir =
            map (causalityHelper dir)
                (pipelineIdentifier
                    </> s "resources"
                    </> string
                    </> s "causality"
                    </> int
                )
    in
    oneOf
        [ baseRoute Concourse.Upstream
            </> s "upstream"
        , baseRoute Concourse.Downstream
            </> s "downstream"
        ]



-- route utils


buildRoute : Int -> String -> Maybe Concourse.JobIdentifier -> Route
buildRoute id name jobId =
    case jobId of
        Just j ->
            Build
                { id =
                    { teamName = j.teamName
                    , pipelineName = j.pipelineName
                    , pipelineInstanceVars = j.pipelineInstanceVars
                    , jobName = j.jobName
                    , buildName = name
                    }
                , highlight = HighlightNothing
                , groups = []
                }

        Nothing ->
            OneOffBuild { id = id, highlight = HighlightNothing }


jobRoute : Concourse.Job -> Route
jobRoute j =
    Job
        { id =
            { teamName = j.teamName
            , pipelineName = j.pipelineName
            , pipelineInstanceVars = j.pipelineInstanceVars
            , jobName = j.name
            }
        , page = Nothing
        , groups = []
        }


resourceRoute : Concourse.ResourceIdentifier -> Maybe Concourse.Version -> Route
resourceRoute r v =
    Resource
        { id =
            { teamName = r.teamName
            , pipelineName = r.pipelineName
            , pipelineInstanceVars = r.pipelineInstanceVars
            , resourceName = r.resourceName
            }
        , page = Nothing
        , version = v
        , groups = []
        }


pipelineRoute : { a | name : String, teamName : String, instanceVars : InstanceVars } -> List String -> Route
pipelineRoute p groups =
    Pipeline
        { id = Concourse.toPipelineId p
        , groups = groups
        }


showHighlight : Highlight -> String
showHighlight hl =
    case hl of
        HighlightNothing ->
            ""

        HighlightLine id line ->
            "#L" ++ id ++ ":" ++ String.fromInt line

        HighlightRange id line1 line2 ->
            "#L"
                ++ id
                ++ ":"
                ++ String.fromInt line1
                ++ ":"
                ++ String.fromInt line2


parseHighlight : Maybe String -> Highlight
parseHighlight hash =
    case hash of
        Just h ->
            case String.uncons h of
                Just ( 'L', selector ) ->
                    case String.split ":" selector of
                        [ stepID, line1str, line2str ] ->
                            case ( String.toInt line1str, String.toInt line2str ) of
                                ( Just line1, Just line2 ) ->
                                    HighlightRange stepID line1 line2

                                _ ->
                                    HighlightNothing

                        [ stepID, linestr ] ->
                            case String.toInt linestr of
                                Just line ->
                                    HighlightLine stepID line

                                _ ->
                                    HighlightNothing

                        _ ->
                            HighlightNothing

                _ ->
                    HighlightNothing

        _ ->
            HighlightNothing


tokenToFlyRoute : String -> Int -> String
tokenToFlyRoute authToken flyPort =
    Builder.crossOrigin
        ("http://127.0.0.1:" ++ String.fromInt flyPort)
        []
        [ Builder.string "token" authToken ]



-- router


sitemap : Parser ((InstanceVars -> Route) -> a) a
sitemap =
    oneOf
        [ resource
        , job
        , dashboard
        , pipeline
        , build
        , oneOffBuild
        , flySuccess
        , causality
        ]


toString : Route -> String
toString route =
    case route of
        Build { id, highlight } ->
            (pipelineIdBuilder id
                |> appendPath [ "jobs", id.jobName, "builds", id.buildName ]
                |> RouteBuilder.build
            )
                ++ showHighlight highlight

        Job { id, page } ->
            pipelineIdBuilder id
                |> appendPath [ "jobs", id.jobName ]
                |> appendQuery (Api.Pagination.params page)
                |> RouteBuilder.build

        Resource { id, page, version } ->
            pipelineIdBuilder id
                |> appendPath [ "resources", id.resourceName ]
                |> appendQuery (Api.Pagination.params page)
                |> appendQuery (Maybe.withDefault Dict.empty version |> versionQueryParams)
                |> RouteBuilder.build

        OneOffBuild { id, highlight } ->
            (( [ "builds", String.fromInt id ], [] )
                |> RouteBuilder.build
            )
                ++ showHighlight highlight

        Pipeline { id, groups } ->
            pipelineIdBuilder id
                |> appendQuery (groups |> List.map (Builder.string "group"))
                |> RouteBuilder.build

        Dashboard { searchType, dashboardView } ->
            ( [], [] )
                |> appendPath
                    (case searchType of
                        Normal _ ->
                            []

                        HighDensity ->
                            [ "hd" ]
                    )
                |> appendQuery
                    (case searchType of
                        Normal "" ->
                            []

                        Normal query ->
                            searchQueryParams query

                        _ ->
                            []
                    )
                |> appendQuery
                    (case dashboardView of
                        ViewNonArchivedPipelines ->
                            []

                        _ ->
                            [ Builder.string "view" <| dashboardViewName dashboardView ]
                    )
                |> RouteBuilder.build

        FlySuccess noop flyPort ->
            ( [ "fly_success" ], [] )
                |> appendQuery
                    (flyPort
                        |> Maybe.map (Builder.int "fly_port")
                        |> Maybe.Extra.toList
                    )
                |> appendQuery
                    (if noop then
                        [ Builder.string "noop" "true" ]

                     else
                        []
                    )
                |> RouteBuilder.build

        Causality { id, direction } ->
            let
                path =
                    case direction of
                        Concourse.Downstream ->
                            "downstream"

                        Concourse.Upstream ->
                            "upstream"
            in
            pipelineIdBuilder id
                |> appendPath [ "resources", id.resourceName ]
                |> appendPath [ "causality", String.fromInt id.versionID ]
                |> appendPath [ path ]
                |> RouteBuilder.build


parsePath : Url.Url -> Maybe Route
parsePath url =
    let
        instanceVars =
            url.query
                |> Maybe.withDefault ""
                |> String.split "&"
                |> List.filter (\s -> String.startsWith "vars." s || String.startsWith "vars=" s)
                |> List.filterMap Url.percentDecode
                |> List.filterMap (DotNotation.parse >> Result.toMaybe)
                |> DotNotation.expand
                |> Dict.get "vars"
                |> toDict
    in
    parse sitemap url |> Maybe.map (\deferredRoute -> deferredRoute instanceVars)


toDict : Maybe JsonValue -> Dict String JsonValue
toDict j =
    case j of
        Just (JsonObject kvs) ->
            Dict.fromList kvs

        _ ->
            Dict.empty



-- route utils


extractPid : Route -> Maybe Concourse.PipelineIdentifier
extractPid route =
    case route of
        Build { id } ->
            Just <| Concourse.pipelineId id

        Job { id } ->
            Just <| Concourse.pipelineId id

        Resource { id } ->
            Just <| Concourse.pipelineId id

        Pipeline { id } ->
            Just id

        _ ->
            Nothing


extractQuery : SearchType -> String
extractQuery route =
    case route of
        Normal q ->
            q

        _ ->
            ""


searchQueryParams : String -> List Builder.QueryParameter
searchQueryParams q =
    [ Builder.string "search" q ]


versionQueryParams : Concourse.Version -> List Builder.QueryParameter
versionQueryParams version =
    Concourse.versionQuery version
        |> List.map (\q -> Builder.string "filter" q)


pipelineIdBuilder : { r | teamName : String, pipelineName : String, pipelineInstanceVars : Concourse.InstanceVars } -> RouteBuilder
pipelineIdBuilder =
    RouteBuilder.pipeline


getGroups : Route -> List String
getGroups route =
    case route of
        Build { groups } ->
            groups

        Resource { groups } ->
            groups

        Job { groups } ->
            groups

        Pipeline { groups } ->
            groups

        Causality { groups } ->
            groups

        OneOffBuild _ ->
            []

        Dashboard _ ->
            []

        FlySuccess _ _ ->
            []


withGroups : List String -> Route -> Route
withGroups groups route =
    case route of
        Build params ->
            Build { params | groups = groups }

        Resource params ->
            Resource { params | groups = groups }

        Job params ->
            Job { params | groups = groups }

        Pipeline params ->
            Pipeline { params | groups = groups }

        Causality params ->
            Causality { params | groups = groups }

        OneOffBuild _ ->
            route

        Dashboard _ ->
            route

        FlySuccess _ _ ->
            route
