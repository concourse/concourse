module BetaResource exposing (Flags, Msg(..), Model, init, changeToResource, update, updateWithMessage, view, subscriptions, PauseChangingOrErrored(..))

import Concourse
import Concourse.Build
import Concourse.BuildStatus
import Concourse.Pagination exposing (Pagination, Paginated, Page, equal)
import Concourse.Resource
import Dict exposing (Dict)
import DictView
import Erl
import Html exposing (Html)
import Html.Attributes exposing (class, href)
import Html.Attributes.Aria exposing (ariaLabel)
import Html.Events exposing (onClick)
import Http
import LoginRedirect
import Navigation
import RemoteData exposing (WebData)
import Set exposing (Set)
import StrictEvents
import Task exposing (Task)
import Time exposing (Time)
import UpdateMsg exposing (UpdateMsg)


type alias Ports =
    { title : String -> Cmd Msg
    }


type alias Model =
    { ports : Ports
    , resourceIdentifier : Concourse.ResourceIdentifier
    , resource : WebData Concourse.Resource
    , pausedChanging : PauseChangingOrErrored
    , versionedResources : Paginated Concourse.VersionedResource
    , currentPage : Maybe Page
    , csrfToken : String
    , versionedUIStates : Dict.Dict Int VersionUIState
    , causality : Dict.Dict Int (WebData (List Concourse.Cause))
    , causalityBuilds : Dict.Dict Int (WebData Concourse.Build)
    , causalityVersions : Dict.Dict Int (WebData Concourse.VersionedResource)
    }


type alias VersionUIState =
    { changingErrored : Bool
    , expanded : Bool
    }


type PauseChangingOrErrored
    = Stable
    | Changing
    | Errored


type Msg
    = Noop
    | AutoupdateTimerTicked Time
    | ResourceFetched (Result Http.Error Concourse.Resource)
    | TogglePaused
    | PausedToggled (Result Http.Error ())
    | VersionedResourcesFetched (Maybe Page) (Result Http.Error (Paginated Concourse.VersionedResource))
    | LoadPage Page
    | ToggleVersionedResource Int
    | VersionedResourceToggled Int (Result Http.Error ())
    | ExpandVersionedResource Int
    | CausalityFetched Int (RemoteData.WebData (List Concourse.Cause))
    | BuildFetched Int (RemoteData.WebData Concourse.Build)
    | VersionedResourceFetched Int (RemoteData.WebData Concourse.VersionedResource)
    | NavTo String


type alias Flags =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , paging : Maybe Concourse.Pagination.Page
    , csrfToken : String
    }


init : Ports -> Flags -> ( Model, Cmd Msg )
init ports flags =
    let
        ( model, cmd ) =
            changeToResource flags
                { resourceIdentifier =
                    { teamName = flags.teamName
                    , pipelineName = flags.pipelineName
                    , resourceName = flags.resourceName
                    }
                , resource = RemoteData.NotAsked
                , pausedChanging = Stable
                , currentPage = Nothing
                , versionedResources =
                    { content = []
                    , pagination =
                        { previousPage = Nothing
                        , nextPage = Nothing
                        }
                    }
                , versionedUIStates = Dict.empty
                , ports = ports
                , csrfToken = flags.csrfToken
                , causality = Dict.empty
                , causalityBuilds = Dict.empty
                , causalityVersions = Dict.empty
                }
    in
        ( model
        , Cmd.batch
            [ fetchResource model.resourceIdentifier
            , cmd
            ]
        )


changeToResource : Flags -> Model -> ( Model, Cmd Msg )
changeToResource flags model =
    ( { model
        | currentPage = flags.paging
        , versionedResources =
            { content = []
            , pagination =
                { previousPage = Nothing
                , nextPage = Nothing
                }
            }
      }
    , fetchVersionedResources model.resourceIdentifier flags.paging
    )


updateWithMessage : Msg -> Model -> ( Model, Cmd Msg, Maybe UpdateMsg )
updateWithMessage message model =
    let
        ( mdl, msg ) =
            update message model
    in
        case mdl.resource of
            RemoteData.Failure _ ->
                ( mdl, msg, Just UpdateMsg.NotFound )

            _ ->
                ( mdl, msg, Nothing )


update : Msg -> Model -> ( Model, Cmd Msg )
update action model =
    case action of
        Noop ->
            ( model, Cmd.none )

        AutoupdateTimerTicked timestamp ->
            ( model
            , Cmd.batch
                [ fetchResource model.resourceIdentifier
                , fetchVersionedResources model.resourceIdentifier model.currentPage
                , updateExpandedProperties model
                ]
            )

        ResourceFetched (Ok resource) ->
            ( { model | resource = RemoteData.Success resource }
            , model.ports.title <| resource.name ++ " - "
            )

        ResourceFetched (Err err) ->
            case Debug.log ("failed to fetch resource") (err) of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else if status.code == 404 then
                        ( { model | resource = RemoteData.Failure err }, Cmd.none )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        TogglePaused ->
            case model.resource |> RemoteData.toMaybe of
                Nothing ->
                    ( model, Cmd.none )

                Just r ->
                    ( { model
                        | pausedChanging = Changing
                        , resource = RemoteData.Success { r | paused = not r.paused }
                      }
                    , if r.paused then
                        unpauseResource model.resourceIdentifier model.csrfToken
                      else
                        pauseResource model.resourceIdentifier model.csrfToken
                    )

        PausedToggled (Ok ()) ->
            ( { model | pausedChanging = Stable }, Cmd.none )

        PausedToggled (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    case model.resource |> RemoteData.toMaybe of
                        Nothing ->
                            ( model, Cmd.none )

                        Just r ->
                            ( { model
                                | pausedChanging = Errored
                                , resource = RemoteData.Success { r | paused = not r.paused }
                              }
                            , Cmd.none
                            )

        VersionedResourcesFetched requestedPage (Ok paginated) ->
            let
                fetchedPage =
                    permalink paginated.content

                newModel =
                    \newPage ->
                        { model
                            | versionedResources = paginated
                            , currentPage = newPage
                        }

                chosenModelWith =
                    \requestedPageUnwrapped ->
                        case model.currentPage of
                            Nothing ->
                                newModel <| Just fetchedPage

                            Just page ->
                                if Concourse.Pagination.equal page requestedPageUnwrapped then
                                    newModel <| requestedPage
                                else
                                    model
            in
                case requestedPage of
                    Nothing ->
                        ( newModel (Just fetchedPage), Cmd.none )

                    Just requestedPageUnwrapped ->
                        ( chosenModelWith requestedPageUnwrapped
                        , Cmd.none
                        )

        VersionedResourcesFetched _ (Err err) ->
            flip always (Debug.log ("failed to fetch versioned resources") (err)) <|
                ( model, Cmd.none )

        LoadPage page ->
            ( { model
                | currentPage = Just page
              }
            , Cmd.batch
                [ fetchVersionedResources model.resourceIdentifier <| Just page
                , Navigation.newUrl <| paginationRoute model.resourceIdentifier page
                ]
            )

        ToggleVersionedResource versionID ->
            let
                versionedResourceIdentifier =
                    versionIdentifier model versionID

                versionedResource =
                    List.head <|
                        List.filter (checkForVersionID versionID) model.versionedResources.content
            in
                ( model
                , case versionedResource of
                    Just vr ->
                        if vr.enabled then
                            disableVersionedResource versionedResourceIdentifier model.csrfToken
                        else
                            enableVersionedResource versionedResourceIdentifier model.csrfToken

                    Nothing ->
                        Cmd.none
                )

        VersionedResourceToggled versionID (Ok ()) ->
            let
                oldState =
                    getState versionID model.versionedUIStates

                newState =
                    { oldState
                        | changingErrored = False
                    }

                oldVRs =
                    model.versionedResources

                oldContent =
                    model.versionedResources.content
            in
                ( { model
                    | versionedResources =
                        { oldVRs
                            | content = updateMatchingMember versionID oldContent
                        }
                    , versionedUIStates = setState versionID newState model.versionedUIStates
                  }
                , Cmd.none
                )

        VersionedResourceToggled versionID (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    let
                        oldState =
                            getState versionID model.versionedUIStates

                        newState =
                            { oldState
                                | expanded = not oldState.expanded
                                , changingErrored = True
                            }
                    in
                        ( { model
                            | versionedUIStates = setState versionID newState model.versionedUIStates
                          }
                        , Cmd.none
                        )

        ExpandVersionedResource versionID ->
            let
                versionedResourceIdentifier =
                    versionIdentifier model versionID

                oldState =
                    getState versionID model.versionedUIStates

                newState =
                    { oldState
                        | expanded = not oldState.expanded
                    }
            in
                ( { model
                    | versionedUIStates = setState versionID newState model.versionedUIStates
                  }
                , if newState.expanded then
                    fetchCausality versionedResourceIdentifier
                  else
                    Cmd.none
                )

        CausalityFetched versionID response ->
            let
                withCausality =
                    { model | causality = Dict.insert versionID response model.causality }

                fetchCausalityDependents cause ( model, cmd ) =
                    conditionallyFetchBuild cause.buildID <|
                        conditionallyFetchVersion cause.versionedResourceID <|
                            ( model, cmd )
            in
                case response of
                    RemoteData.Success causality ->
                        List.foldl fetchCausalityDependents
                            ( withCausality, Cmd.none )
                            causality

                    _ ->
                        ( withCausality, Cmd.none )

        BuildFetched buildID response ->
            ( { model | causalityBuilds = Dict.insert buildID response model.causalityBuilds }
            , Cmd.none
            )

        VersionedResourceFetched vrID response ->
            ( { model | causalityVersions = Dict.insert vrID response model.causalityVersions }
            , Cmd.none
            )

        NavTo url ->
            ( model, Navigation.newUrl url )


conditionallyFetchBuild : Int -> ( Model, Cmd Msg ) -> ( Model, Cmd Msg )
conditionallyFetchBuild id ( model, cmd ) =
    case getData id model.causalityBuilds of
        RemoteData.Success build ->
            if Concourse.BuildStatus.isRunning build.status then
                ( model, Cmd.batch [ fetchBuild id, cmd ] )
            else
                ( model, cmd )

        RemoteData.NotAsked ->
            ( { model | causalityBuilds = Dict.insert id RemoteData.Loading model.causalityBuilds }
            , Cmd.batch [ fetchBuild id, cmd ]
            )

        _ ->
            ( model, cmd )


conditionallyFetchVersion : Int -> ( Model, Cmd Msg ) -> ( Model, Cmd Msg )
conditionallyFetchVersion id ( model, cmd ) =
    case getData id model.causalityVersions of
        RemoteData.NotAsked ->
            ( { model | causalityVersions = Dict.insert id RemoteData.Loading model.causalityVersions }
            , Cmd.batch [ fetchVersionedResource (versionIdentifier model id), cmd ]
            )

        _ ->
            ( model, cmd )


versionIdentifier : Model -> Int -> Concourse.VersionedResourceIdentifier
versionIdentifier model versionedResourceID =
    { teamName = model.resourceIdentifier.teamName
    , pipelineName = model.resourceIdentifier.pipelineName
    , resourceName = model.resourceIdentifier.resourceName
    , versionID = versionedResourceID
    }


permalink : List Concourse.VersionedResource -> Page
permalink versionedResources =
    case List.head versionedResources of
        Nothing ->
            { direction = Concourse.Pagination.Since 0
            , limit = 100
            }

        Just version ->
            { direction = Concourse.Pagination.From version.id
            , limit = List.length versionedResources
            }


paginationRoute : Concourse.ResourceIdentifier -> Page -> String
paginationRoute rid page =
    let
        ( param, boundary ) =
            case page.direction of
                Concourse.Pagination.Since bound ->
                    ( "since", Basics.toString bound )

                Concourse.Pagination.Until bound ->
                    ( "until", Basics.toString bound )

                Concourse.Pagination.From bound ->
                    ( "from", Basics.toString bound )

                Concourse.Pagination.To bound ->
                    ( "to", Basics.toString bound )

        parsedRoute =
            Erl.parse <|
                "/teams/"
                    ++ rid.teamName
                    ++ "/pipelines/"
                    ++ rid.pipelineName
                    ++ "/resources/"
                    ++ rid.resourceName

        newParsedRoute =
            Erl.addQuery param boundary <| Erl.addQuery "limit" (Basics.toString page.limit) parsedRoute
    in
        Erl.toString newParsedRoute


view : Model -> Html Msg
view model =
    case model.resource |> RemoteData.toMaybe of
        Just resource ->
            let
                ( checkStatus, checkMessage, stepBody ) =
                    if resource.failingToCheck then
                        ( "fr errored fa fa-fw fa-exclamation-triangle"
                        , "checking failed"
                        , [ Html.div [ class "step-body" ]
                                [ Html.pre [] [ Html.text resource.checkError ]
                                ]
                          ]
                        )
                    else
                        ( "fr succeeded fa fa-fw fa-check", "checking successfully", [] )

                ( paused, pausedIcon, aria, onClickEvent ) =
                    case ( resource.paused, model.pausedChanging ) of
                        ( _, Changing ) ->
                            ( "loading", "fa-spin fa-circle-o-notch", "", Noop )

                        ( True, Errored ) ->
                            ( "errored", "fa-play", "Resume Resource Checking", TogglePaused )

                        ( False, Errored ) ->
                            ( "errored", "fa-pause", "Pause Resource Checking", TogglePaused )

                        ( True, Stable ) ->
                            ( "enabled", "fa-play", "Resume Resource Checking", TogglePaused )

                        ( False, Stable ) ->
                            ( "disabled", "fa-pause", "Pause Resource Checking", TogglePaused )

                ( previousButtonClass, previousButtonEvent ) =
                    case model.versionedResources.pagination.previousPage of
                        Nothing ->
                            ( "btn-page-link disabled", Noop )

                        Just pp ->
                            ( "btn-page-link", LoadPage pp )

                ( nextButtonClass, nextButtonEvent ) =
                    case model.versionedResources.pagination.nextPage of
                        Nothing ->
                            ( "btn-page-link disabled", Noop )

                        Just np ->
                            let
                                updatedPage =
                                    { np
                                        | limit = 100
                                    }
                            in
                                ( "btn-page-link", LoadPage updatedPage )
            in
                Html.div [ class "with-fixed-header" ]
                    [ Html.div [ class "fixed-header" ]
                        [ Html.div [ class "pagination-header" ]
                            [ Html.div [ class "pagination fr" ]
                                [ Html.div [ class previousButtonClass, onClick previousButtonEvent ]
                                    [ Html.a [ class "arrow" ]
                                        [ Html.i [ class "fa fa-arrow-left" ] []
                                        ]
                                    ]
                                , Html.div [ class nextButtonClass, onClick nextButtonEvent ]
                                    [ Html.a [ class "arrow" ]
                                        [ Html.i [ class "fa fa-arrow-right" ] []
                                        ]
                                    ]
                                ]
                            , Html.h1 [] [ Html.text resource.name ]
                            ]
                        ]
                    , Html.div [ class "scrollable-body" ]
                        [ Html.div [ class "resource-check-status" ]
                            [ Html.div [ class "build-step" ]
                                (List.append
                                    [ Html.div [ class "header" ]
                                        [ Html.span
                                            [ class <| "btn-pause fl " ++ paused
                                            , ariaLabel aria
                                            , onClick onClickEvent
                                            ]
                                            [ Html.i [ class <| "fa fa-fw " ++ pausedIcon ] []
                                            ]
                                        , Html.h3 [] [ Html.text checkMessage ]
                                        , Html.i [ class <| checkStatus ] []
                                        ]
                                    ]
                                    stepBody
                                )
                            ]
                        , (viewVersionedResources model model.versionedResources.content model.versionedUIStates)
                        ]
                    ]

        Nothing ->
            Html.div [] []


checkForVersionID : Int -> Concourse.VersionedResource -> Bool
checkForVersionID versionID versionedResource =
    versionID == versionedResource.id


updateMatchingMember : Int -> List Concourse.VersionedResource -> List Concourse.VersionedResource
updateMatchingMember versionID versionedResources =
    List.map (switchEnabled versionID) versionedResources


switchEnabled : Int -> Concourse.VersionedResource -> Concourse.VersionedResource
switchEnabled versionID versionedResource =
    let
        wasEnabled =
            versionedResource.enabled
    in
        if versionID == versionedResource.id then
            { versionedResource
                | enabled = not wasEnabled
            }
        else
            versionedResource


viewVersionedResources : Model -> List Concourse.VersionedResource -> Dict.Dict Int VersionUIState -> Html Msg
viewVersionedResources model versionedResources states =
    Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]
        (List.map (viewVersionedResource model states) versionedResources)


viewVersionedResource : Model -> Dict.Dict Int VersionUIState -> Concourse.VersionedResource -> Html Msg
viewVersionedResource model states versionedResource =
    let
        resourceState =
            getState versionedResource.id states

        expanded =
            if resourceState.expanded then
                " expanded"
            else
                ""

        liEnabled =
            (if resourceState.changingErrored then
                "errored "
             else
                ""
            )
                ++ (if versionedResource.enabled then
                        "enabled"
                    else
                        "disabled"
                   )
                ++ expanded
    in
        Html.li [ class <| "list-collapsable-item clearfix " ++ liEnabled ]
            [ Html.a
                [ class "fl btn-power-toggle fa fa-power-off mrm"
                , ariaLabel "Toggle Resource Version"
                , onClick <| ToggleVersionedResource versionedResource.id
                ]
                []
            , Html.div [ class "list-collapsable-title", onClick <| ExpandVersionedResource versionedResource.id ]
                [ viewVersion versionedResource.version ]
            , Html.div [ class "list-collapsable-content clearfix" ]
                [ Html.div [ class "causality" ]
                    [ case getData versionedResource.id model.causality of
                        RemoteData.Success causality ->
                            viewCausality model causality

                        x ->
                            Html.text (toString x)
                    ]
                , Html.div [ class "vri metadata-container" ]
                    [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
                    , viewMetadata versionedResource.metadata
                    ]
                ]
            ]


getState : Int -> Dict.Dict Int VersionUIState -> VersionUIState
getState versionID states =
    let
        resourceState =
            Dict.get versionID states
    in
        case resourceState of
            Nothing ->
                { changingErrored = False
                , expanded = False
                }

            Just rs ->
                rs


setState : Int -> VersionUIState -> Dict.Dict Int VersionUIState -> Dict.Dict Int VersionUIState
setState versionID newState states =
    Dict.insert versionID newState states


viewVersion : Concourse.Version -> Html Msg
viewVersion version =
    DictView.view
        << Dict.map (\_ s -> Html.text s)
    <|
        version


viewCausality : Model -> List Concourse.Cause -> Html Msg
viewCausality model causality =
    Html.div [ class "causality-sequence" ] <|
        viewCauses model causality Set.empty Set.empty


viewCauses : Model -> List Concourse.Cause -> Set Int -> Set Int -> List (Html Msg)
viewCauses model causes seenVersions seenBuilds =
    case causes of
        [] ->
            []

        { versionedResourceID, buildID } :: rest ->
            let
                build =
                    viewCauseBuild model buildID

                version =
                    viewCauseVersion model versionedResourceID

                viewRest =
                    viewCauses model
                        rest
                        (Set.insert versionedResourceID seenVersions)
                        (Set.insert buildID seenBuilds)
            in
                case ( Set.member versionedResourceID seenVersions, Set.member buildID seenBuilds ) of
                    ( True, True ) ->
                        viewRest

                    ( False, True ) ->
                        [ version ] ++ viewRest

                    ( True, False ) ->
                        [ build ] ++ viewRest

                    ( False, False ) ->
                        [ version, build ] ++ viewRest


viewCauseBuild : Model -> Int -> Html Msg
viewCauseBuild model buildID =
    case getData buildID model.causalityBuilds of
        RemoteData.Success build ->
            viewBuildCausality build

        x ->
            Html.text (toString x)


viewCauseVersion : Model -> Int -> Html Msg
viewCauseVersion model versionedResourceID =
    case getData versionedResourceID model.causalityVersions of
        RemoteData.Success version ->
            viewVersionCausality version

        x ->
            Html.text (toString x)


viewBuildCausality : Concourse.Build -> Html Msg
viewBuildCausality build =
    let
        jobName =
            Maybe.withDefault "one-off" <| Maybe.map .jobName build.job
    in
        Html.div [ class <| "causality-build " ++ Concourse.BuildStatus.show build.status ]
            [ Html.a
                [ StrictEvents.onLeftClick <| NavTo build.url
                , href build.url
                ]
                [ Html.text <| jobName ++ " #" ++ build.name ]
            ]


viewVersionCausality : Concourse.VersionedResource -> Html Msg
viewVersionCausality vr =
    Html.div [ class "causality-resource" ]
        [ Html.span [ class "resource-name" ]
            [ Html.text vr.resourceName
            , Html.text " @ "
            ]
        , viewVersion vr.version
        ]


getData : comparable -> Dict comparable (RemoteData.WebData x) -> RemoteData.WebData x
getData a dict =
    Maybe.withDefault RemoteData.NotAsked <| Dict.get a dict


viewMetadata : Concourse.Metadata -> Html Msg
viewMetadata metadata =
    Html.dl [ class "build-metadata" ]
        (List.concatMap viewMetadataField metadata)


viewMetadataField : Concourse.MetadataField -> List (Html a)
viewMetadataField field =
    [ Html.dt [] [ Html.text field.name ]
    , Html.dd []
        [ Html.pre [ class "metadata-field" ] [ Html.text field.value ]
        ]
    ]


listToMap : List Concourse.Build -> Dict.Dict String (List Concourse.Build)
listToMap builds =
    let
        insertBuild =
            \build dict ->
                let
                    jobName =
                        case build.job of
                            Nothing ->
                                Debug.crash "Jobless builds shouldn't appear on this page!" ""

                            Just job ->
                                job.jobName

                    oldList =
                        Dict.get jobName dict

                    newList =
                        case oldList of
                            Nothing ->
                                [ build ]

                            Just list ->
                                list ++ [ build ]
                in
                    Dict.insert jobName newList dict
    in
        List.foldr insertBuild Dict.empty builds


viewBuilds : Dict.Dict String (List Concourse.Build) -> List (Html Msg)
viewBuilds buildDict =
    List.concatMap (viewBuildsByJob buildDict) <| Dict.keys buildDict


viewBuildsByJob : Dict.Dict String (List Concourse.Build) -> String -> List (Html Msg)
viewBuildsByJob buildDict jobName =
    let
        oneBuildToLi =
            \build ->
                let
                    link =
                        case build.job of
                            Nothing ->
                                ""

                            Just job ->
                                "/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds/" ++ build.name
                in
                    Html.li [ class <| Concourse.BuildStatus.show build.status ]
                        [ Html.a
                            [ StrictEvents.onLeftClick <| NavTo link
                            , href link
                            ]
                            [ Html.text <| "#" ++ build.name ]
                        ]
    in
        [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
        , Html.ul [ class "builds-list" ]
            (case (Dict.get jobName buildDict) of
                Nothing ->
                    []

                -- never happens
                Just buildList ->
                    (List.map oneBuildToLi buildList)
            )
        ]


updateExpandedProperties : Model -> Cmd Msg
updateExpandedProperties model =
    let
        filteredList =
            List.filter
                (isExpanded model.versionedUIStates)
                model.versionedResources.content
    in
        Cmd.batch <|
            List.map (fetchVersionCausality model) filteredList


isExpanded : Dict.Dict Int VersionUIState -> Concourse.VersionedResource -> Bool
isExpanded states versionedResource =
    let
        state =
            Dict.get versionedResource.id states
    in
        case state of
            Nothing ->
                False

            Just someState ->
                someState.expanded


fetchVersionCausality : Model -> Concourse.VersionedResource -> Cmd Msg
fetchVersionCausality model versionedResource =
    fetchCausality (versionIdentifier model versionedResource.id)


fetchResource : Concourse.ResourceIdentifier -> Cmd Msg
fetchResource resourceIdentifier =
    Task.attempt ResourceFetched <|
        Concourse.Resource.fetchResource resourceIdentifier


pauseResource : Concourse.ResourceIdentifier -> Concourse.CSRFToken -> Cmd Msg
pauseResource resourceIdentifier csrfToken =
    Task.attempt PausedToggled <|
        Concourse.Resource.pause resourceIdentifier csrfToken


unpauseResource : Concourse.ResourceIdentifier -> Concourse.CSRFToken -> Cmd Msg
unpauseResource resourceIdentifier csrfToken =
    Task.attempt PausedToggled <|
        Concourse.Resource.unpause resourceIdentifier csrfToken


fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Cmd Msg
fetchVersionedResources resourceIdentifier page =
    Task.attempt (VersionedResourcesFetched page) <|
        Concourse.Resource.fetchVersionedResources resourceIdentifier page


enableVersionedResource : Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Cmd Msg
enableVersionedResource versionedResourceIdentifier csrfToken =
    Task.attempt (VersionedResourceToggled versionedResourceIdentifier.versionID) <|
        Concourse.Resource.enableVersionedResource versionedResourceIdentifier csrfToken


disableVersionedResource : Concourse.VersionedResourceIdentifier -> Concourse.CSRFToken -> Cmd Msg
disableVersionedResource versionedResourceIdentifier csrfToken =
    Task.attempt (VersionedResourceToggled versionedResourceIdentifier.versionID) <|
        Concourse.Resource.disableVersionedResource versionedResourceIdentifier csrfToken


fetchCausality : Concourse.VersionedResourceIdentifier -> Cmd Msg
fetchCausality versionedResourceIdentifier =
    Cmd.map (CausalityFetched versionedResourceIdentifier.versionID) <|
        RemoteData.asCmd <|
            Concourse.Resource.fetchCausality versionedResourceIdentifier


fetchBuild : Int -> Cmd Msg
fetchBuild buildID =
    Cmd.map (BuildFetched buildID)
        << RemoteData.asCmd
    <|
        Concourse.Build.fetch buildID


fetchVersionedResource : Concourse.VersionedResourceIdentifier -> Cmd Msg
fetchVersionedResource versionedResourceIdentifier =
    Cmd.map (VersionedResourceFetched versionedResourceIdentifier.versionID)
        << RemoteData.asCmd
    <|
        Concourse.Resource.fetchVersionedResource versionedResourceIdentifier


subscriptions : Model -> Sub Msg
subscriptions model =
    Time.every (5 * Time.second) AutoupdateTimerTicked
