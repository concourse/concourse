module Causality.Causality exposing
    ( Build
    , Model
    , NodeType(..)
    , Version
    , changeToVersionedResource
    , constructGraph
    , documentTitle
    , getUpdateMessage
    , graphvizDotNotation
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Causality.DOT as DOT exposing (Attr(..))
import ColorValues
import Colors exposing (buildStatusColor)
import Concourse
    exposing
        ( Causality
        , CausalityDirection(..)
        )
import Concourse.BuildStatus exposing (BuildStatus(..))
import Dict exposing (Dict)
import EffectTransformer exposing (ET)
import Graph exposing (Edge, Graph, Node)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , id
        , style
        )
import Http
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Routes
import SideBar.SideBar as SideBar
import Svg
import Svg.Attributes as SvgAttributes
import Tooltip
import UpdateMsg exposing (UpdateMsg)
import Views.Styles
import Views.TopBar as TopBar


type PageError
    = NotFound
    | TooManyNodes
    | NoNodes


type alias Model =
    Login.Model
        { versionId : Concourse.VersionedResourceIdentifier
        , direction : CausalityDirection
        , fetchedVersionedResource : Maybe Concourse.VersionedResource
        , fetchedCausality : Maybe Causality
        , graph : Graph NodeType ()
        , renderedJobs : Maybe (List Concourse.Job)
        , renderedBuilds : Maybe (List Concourse.Build)
        , renderedResources : Maybe (List Concourse.Resource)
        , renderedResourceVersions : Maybe (List Concourse.VersionedResource)
        , pageStatus : Result PageError ()
        }


type alias Flags =
    { versionId : Concourse.VersionedResourceIdentifier
    , direction : CausalityDirection
    }


documentTitle : Model -> String
documentTitle model =
    model.versionId.resourceName


init : Flags -> ( Model, List Effect )
init flags =
    let
        fetchCausality =
            case flags.direction of
                Concourse.Downstream ->
                    FetchDownstreamCausality flags.versionId

                Concourse.Upstream ->
                    FetchUpstreamCausality flags.versionId
    in
    ( { isUserMenuExpanded = False
      , versionId = flags.versionId
      , direction = flags.direction
      , fetchedVersionedResource = Nothing
      , fetchedCausality = Nothing
      , graph = Graph.empty
      , renderedJobs = Nothing
      , renderedBuilds = Nothing
      , renderedResources = Nothing
      , renderedResourceVersions = Nothing
      , pageStatus = Ok ()
      }
    , [ FetchAllPipelines
      , FetchVersionedResource flags.versionId
      , fetchCausality
      ]
    )


changeToVersionedResource : Flags -> ET Model
changeToVersionedResource flags ( _, effects ) =
    let
        ( newModel, newEffects ) =
            init flags
    in
    ( newModel, effects ++ newEffects )


subscriptions : List Subscription
subscriptions =
    []


tooltip : Model -> Session -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    case callback of
        CausalityFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else if status.code == 422 then
                        ( { model | pageStatus = Err TooManyNodes }, effects )

                    else if status.code == 403 then
                        ( { model | pageStatus = Err NotFound }, effects )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err NotFound }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        CausalityFetched (Ok ( direction, causality )) ->
            let
                graph =
                    case causality of
                        Just c ->
                            constructGraph direction c

                        _ ->
                            model.graph
            in
            if Graph.isEmpty graph then
                ( { model
                    | fetchedCausality = causality
                    , graph = graph
                    , pageStatus = Err NoNodes
                  }
                , effects
                )

            else
                ( { model
                    | fetchedCausality = causality
                    , graph = graph
                  }
                , effects
                    ++ [ RenderCausality <| graphvizDotNotation model graph ]
                )

        VersionedResourceFetched (Ok vr) ->
            ( { model
                | fetchedVersionedResource = Just vr
              }
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        _ ->
            ( model, effects )


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.pageStatus of
        Err NotFound ->
            UpdateMsg.NotFound

        Err _ ->
            UpdateMsg.AOK

        Ok () ->
            UpdateMsg.AOK


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Causality
                { id = model.versionId
                , direction = model.direction
                , version = Maybe.map .version model.fetchedVersionedResource
                , groups = Routes.getGroups session.route
                }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            (SideBar.sideBarIcon session
                :: TopBar.breadcrumbs session route
                ++ [ Login.view session.userState model ]
            )
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (Just
                    { pipelineName = model.versionId.pipelineName
                    , pipelineInstanceVars = model.versionId.pipelineInstanceVars
                    , teamName = model.versionId.teamName
                    }
                )
            , viewGraph model
            ]
        ]


viewGraph : Model -> Html Message
viewGraph model =
    case model.pageStatus of
        Err TooManyNodes ->
            Html.div
                [ class "notfound"
                , id "causality-error"
                ]
                [ Html.div [ class "title" ] [ Html.text "graph too large" ]
                , Html.div [ class "reason" ] [ Html.text "number of builds/resource versions exceeds 5000/25000" ]
                , Html.div [ class "help-message" ]
                    [ Html.text "try a less popular resource version ¯\\_(ツ)_/¯"
                    ]
                ]

        Err NoNodes ->
            Html.div
                [ class "notfound"
                , id "causality-error"
                ]
                [ Html.div [ class "title" ] [ Html.text "no causality" ]
                , Html.div [ class "reason" ] [ Html.text "resource version was not used in any builds" ]
                , Html.div [ class "help-message" ]
                    [ Html.text "try a more popular resource version ¯\\_(ツ)_/¯"
                    ]
                ]

        _ ->
            Html.div
                [ class "causality-view"
                , id "causality-container"
                , style "display" "flex"
                , style "flex-direction" "column"
                , style "flex-grow" "1"
                ]
                [ Html.div
                    [ class "causality-content" ]
                    [ Svg.svg
                        [ SvgAttributes.class "causality-graph" ]
                        []
                    ]
                ]


type alias Build =
    { id : Int
    , name : String
    , status : Concourse.BuildStatus.BuildStatus
    }


type alias Version =
    { id : Int
    , version : Concourse.Version
    }


type NodeType
    = Job String (List Build)
    | Resource String (List Version)


constructGraph : CausalityDirection -> Causality -> Graph NodeType ()
constructGraph direction causality =
    let
        idPairs : List { a | id : Int } -> Dict Int { a | id : Int }
        idPairs =
            Dict.fromList << List.map (\thing -> ( thing.id, thing ))

        fetchIds : (a -> b) -> Dict Int a -> List Int -> List b
        fetchIds fn dict =
            List.filterMap (\id -> Dict.get id dict)
                >> List.map fn

        convertBuild { id, name, status } =
            { id = id
            , name = name
            , status = status
            }

        convertVersion { id, version } =
            { id = id
            , version = version
            }

        builds =
            idPairs causality.builds

        resourceVersions =
            idPairs causality.resourceVersions

        jobNodes =
            List.map
                (\job ->
                    Node -job.id
                        (Job job.name
                            (fetchIds convertBuild builds job.buildIds
                                |> List.sortBy .name
                                |> List.reverse
                            )
                        )
                )
                causality.jobs

        resourceNodes =
            List.map
                (\resource ->
                    Node resource.id
                        (Resource resource.name
                            (fetchIds convertVersion resourceVersions resource.resourceVersionIds
                                |> List.sortBy .id
                                |> List.reverse
                            )
                        )
                )
                causality.resources

        jobEdges =
            List.concatMap
                (\build ->
                    List.map
                        (\vId ->
                            ( -build.jobId
                            , Dict.get vId resourceVersions
                                |> Maybe.map .resourceId
                                |> Maybe.withDefault 0
                            )
                        )
                        build.resourceVersionIds
                )
                causality.builds

        resourceEdges =
            List.concatMap
                (\version ->
                    List.map
                        (\bId ->
                            ( version.resourceId
                            , Dict.get bId builds
                                |> Maybe.map (\b -> -b.jobId)
                                |> Maybe.withDefault 0
                            )
                        )
                        version.buildIds
                )
                causality.resourceVersions

        nodes =
            resourceNodes ++ jobNodes

        pairs =
            resourceEdges ++ jobEdges

        edges =
            case direction of
                Downstream ->
                    List.map (\( a, b ) -> Edge a b ()) pairs

                Upstream ->
                    List.map (\( a, b ) -> Edge b a ()) pairs
    in
    Graph.fromNodesAndEdges nodes edges



-- note: '&' needs to be escaped first, otherwise it'll start escaping itself


escape : String -> String
escape =
    String.replace "&" "&amp;"
        >> String.replace "<" "&lt;"
        >> String.replace ">" "&gt;"
        >> String.replace "\"" "&quot;"
        >> String.replace "'" "&#039;"



-- http://www.graphviz.org/doc/info/shapes.html#html. this should probably use Json.Encode.string to sanitize the output


attributes : List ( String, String ) -> String
attributes =
    List.map (\( k, v ) -> escape k ++ "=\"" ++ escape v ++ "\"") >> String.join " "


graphvizDotNotation : Model -> Graph NodeType () -> String
graphvizDotNotation model =
    let
        -- http://www.graphviz.org/doc/info/attrs.html
        styles : DOT.Styles
        styles =
            { rankdir = DOT.LR
            , graph =
                attributes
                    [ ( "bgcolor", "transparent" )
                    ]
            , node =
                attributes
                    [ ( "color", ColorValues.grey100 )
                    , ( "style", "filled" )
                    , ( "tooltip", " " )
                    , ( "fontname", "Courier" )
                    , ( "fontcolor", Colors.white )
                    ]
            , edge =
                attributes
                    [ ( "color", ColorValues.grey50 )
                    , ( "penwidth", "2.0" )
                    ]
            }

        table body =
            "<TABLE "
                ++ attributes
                    [ ( "BORDER", "0" )
                    , ( "CELLBORDER", "0" )
                    , ( "CELLSPACING", "0" )
                    ]
                ++ ">"
                ++ String.join "" body
                ++ "</TABLE>"

        row attrs body =
            "<TR><TD " ++ attrs ++ ">" ++ body ++ "</TD></TR>"

        { teamName, pipelineName, pipelineInstanceVars } =
            model.versionId

        jobLabel : String -> List Build -> String
        jobLabel name builds =
            table <|
                row "" (escape name)
                    :: List.map
                        (\b ->
                            let
                                build =
                                    { teamName = teamName
                                    , pipelineName = pipelineName
                                    , pipelineInstanceVars = pipelineInstanceVars
                                    , jobName = name
                                    , buildName = b.name
                                    }

                                link =
                                    Routes.Build { id = build, highlight = Routes.HighlightNothing, groups = [] }
                                        |> Routes.toString
                            in
                            row (attributes [ ( "HREF", link ), ( "BGCOLOR", buildStatusColor True b.status ) ]) ("#" ++ b.name)
                        )
                        builds

        resourceLabel : String -> List Version -> String
        resourceLabel name versions =
            table <|
                row "" (escape name)
                    :: List.map
                        (\{ version } ->
                            let
                                versionStr =
                                    Concourse.versionQuery version
                                        |> List.map
                                            (\s ->
                                                escape <|
                                                    if String.length s > 40 then
                                                        String.left 38 s ++ "…"

                                                    else
                                                        s
                                            )
                                        |> String.join "<BR/>"

                                resource =
                                    { teamName = teamName
                                    , pipelineName = pipelineName
                                    , pipelineInstanceVars = pipelineInstanceVars
                                    , resourceName = name
                                    }

                                link =
                                    Routes.resourceRoute resource (Just version)
                                        |> Routes.toString
                            in
                            row
                                (attributes
                                    [ ( "HREF", link )
                                    , ( "BORDER", "4" )
                                    , ( "COLOR", ColorValues.grey80 )
                                    , ( "SIDES", "T" )
                                    ]
                                )
                                versionStr
                        )
                        versions

        nodeAttrs typ =
            Dict.fromList <|
                case typ of
                    Job name builds ->
                        [ ( "class", EscString "job" )
                        , ( "shape", EscString "rect" )
                        , ( "label", HtmlLabel <| jobLabel name builds )
                        ]

                    Resource name versions ->
                        [ ( "class", EscString "resource" )
                        , ( "shape", EscString "rect" )
                        , ( "style", EscString "filled,rounded" )
                        , ( "label", HtmlLabel <| resourceLabel name versions )
                        ]

        edgeAttrs _ =
            Dict.empty
    in
    DOT.outputWithStylesAndAttributes styles nodeAttrs edgeAttrs
