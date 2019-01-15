module Resource exposing
    ( Flags
    , changeToResource
    , init
    , subscriptions
    , update
    , updateWithMessage
    , view
    , viewPinButton
    , viewVersionBody
    , viewVersionHeader
    )

import BoolTransitionable
import Colors
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , Pagination
        , chevron
        , chevronContainer
        , equal
        )
import Css
import Date exposing (Date)
import Date.Format
import Dict
import DictView
import Duration exposing (Duration)
import Erl
import Html.Attributes
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes
    exposing
        ( attribute
        , class
        , css
        , href
        , id
        , style
        , title
        )
import Html.Styled.Events
    exposing
        ( onClick
        , onMouseEnter
        , onMouseLeave
        , onMouseOut
        , onMouseOver
        )
import Http
import List.Extra
import Maybe.Extra as ME
import NewTopBar.Styles as Styles
import Pinned
    exposing
        ( ResourcePinState(..)
        , VersionPinState(..)
        )
import QueryString
import Resource.Effects as Effects exposing (Effect(..))
import Resource.Models as Models exposing (Model)
import Resource.Msgs exposing (Msg(..))
import Resource.Styles
import Routes
import Spinner
import StrictEvents
import Time exposing (Time)
import TopBar
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState(..))


type alias Flags =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , paging : Maybe Concourse.Pagination.Page
    , csrfToken : String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( model, effect ) =
            changeToResource flags
                { resourceIdentifier =
                    { teamName = flags.teamName
                    , pipelineName = flags.pipelineName
                    , resourceName = flags.resourceName
                    }
                , pageStatus = Err Models.Empty
                , teamName = flags.teamName
                , pipelineName = flags.pipelineName
                , name = flags.resourceName
                , checkStatus = Models.CheckingSuccessfully
                , checkError = ""
                , checkSetupError = ""
                , hovered = Models.None
                , lastChecked = Nothing
                , pinnedVersion = NotPinned
                , currentPage = Nothing
                , versions =
                    { content = []
                    , pagination =
                        { previousPage = Nothing
                        , nextPage = Nothing
                        }
                    }
                , now = Nothing
                , csrfToken = flags.csrfToken
                , showPinBarTooltip = False
                , pinIconHover = False
                , route =
                    { logical =
                        Routes.Resource
                            flags.teamName
                            flags.pipelineName
                            flags.resourceName
                    , queries = QueryString.empty
                    , page = Nothing
                    , hash = ""
                    }
                , pipeline = Nothing
                , userState = UserStateUnknown
                , userMenuVisible = False
                , pinnedResources = []
                , showPinIconDropDown = False
                }
    in
    ( model
    , [ FetchResource model.resourceIdentifier
      , DoTopBarUpdate (TopBar.FetchUser 0) model
      , effect
      ]
    )


changeToResource : Flags -> Model -> ( Model, Effect )
changeToResource flags model =
    ( { model
        | currentPage = flags.paging
        , versions =
            { content = []
            , pagination =
                { previousPage = Nothing
                , nextPage = Nothing
                }
            }
      }
    , FetchVersionedResources model.resourceIdentifier flags.paging
    )


updateWithMessage : Msg -> Model -> ( Model, Cmd Msg, Maybe UpdateMsg )
updateWithMessage message model =
    let
        ( mdl, effects ) =
            update message model

        cmd =
            Cmd.batch <| List.map Effects.runEffect effects
    in
    if mdl.pageStatus == Err Models.NotFound then
        ( mdl, cmd, Just UpdateMsg.NotFound )

    else
        ( mdl, cmd, Nothing )


updatePinnedVersion : Concourse.Resource -> Model -> Model
updatePinnedVersion resource model =
    case ( resource.pinnedVersion, resource.pinnedInConfig ) of
        ( Nothing, _ ) ->
            case model.pinnedVersion of
                PinningTo _ ->
                    model

                _ ->
                    { model | pinnedVersion = NotPinned }

        ( Just v, True ) ->
            { model | pinnedVersion = PinnedStaticallyTo v }

        ( Just newVersion, False ) ->
            case model.pinnedVersion of
                UnpinningFrom _ ->
                    { model | pinnedVersion = UnpinningFrom newVersion }

                _ ->
                    { model | pinnedVersion = PinnedDynamicallyTo newVersion }


hasPinnedVersion : Model -> Concourse.Version -> Bool
hasPinnedVersion model v =
    case model.pinnedVersion of
        PinnedStaticallyTo pv ->
            v == pv

        PinnedDynamicallyTo pv ->
            v == pv

        UnpinningFrom pv ->
            v == pv

        _ ->
            False


update : Msg -> Model -> ( Model, List Effect )
update action model =
    case action of
        Noop ->
            ( model, [] )

        AutoupdateTimerTicked timestamp ->
            ( model
            , [ FetchResource model.resourceIdentifier
              , FetchVersionedResources model.resourceIdentifier model.currentPage
              ]
                ++ updateExpandedProperties model
            )

        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , teamName = resource.teamName
                , pipelineName = resource.pipelineName
                , name = resource.name
                , checkStatus =
                    if resource.failingToCheck then
                        Models.FailingToCheck

                    else
                        Models.CheckingSuccessfully
                , checkError = resource.checkError
                , checkSetupError = resource.checkSetupError
                , lastChecked = resource.lastChecked
              }
                |> updatePinnedVersion resource
            , [ SetTitle <| resource.name ++ " - " ]
            )

        ResourceFetched (Err err) ->
            case Debug.log "failed to fetch resource" err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err Models.NotFound }, [] )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        VersionedResourcesFetched requestedPage (Ok paginated) ->
            let
                fetchedPage =
                    permalink paginated.content

                versions =
                    { pagination = paginated.pagination
                    , content =
                        paginated.content
                            |> List.map
                                (\vr ->
                                    let
                                        existingVersion : Maybe Models.Version
                                        existingVersion =
                                            model.versions.content
                                                |> List.Extra.find (\v -> v.id == vr.id)

                                        enabledStateAccordingToServer : BoolTransitionable.BoolTransitionable
                                        enabledStateAccordingToServer =
                                            if vr.enabled then
                                                BoolTransitionable.True

                                            else
                                                BoolTransitionable.False
                                    in
                                    case existingVersion of
                                        Just ev ->
                                            { ev
                                                | enabled =
                                                    if ev.enabled == BoolTransitionable.Changing then
                                                        BoolTransitionable.Changing

                                                    else
                                                        enabledStateAccordingToServer
                                            }

                                        Nothing ->
                                            { id = vr.id
                                            , version = vr.version
                                            , metadata = vr.metadata
                                            , enabled = enabledStateAccordingToServer
                                            , expanded = False
                                            , inputTo = []
                                            , outputOf = []
                                            , showTooltip = False
                                            }
                                )
                    }

                newModel =
                    \newPage ->
                        { model
                            | versions = versions
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
                    ( newModel (Just fetchedPage), [] )

                Just requestedPageUnwrapped ->
                    ( chosenModelWith requestedPageUnwrapped
                    , []
                    )

        VersionedResourcesFetched _ (Err err) ->
            flip always (Debug.log "failed to fetch versioned resources" err) <|
                ( model, [] )

        LoadPage page ->
            ( { model
                | currentPage = Just page
              }
            , [ FetchVersionedResources model.resourceIdentifier <| Just page
              , NewUrl <| paginationRoute model.resourceIdentifier page
              ]
            )

        ExpandVersionedResource versionID ->
            let
                versionedResourceIdentifier =
                    { teamName = model.resourceIdentifier.teamName
                    , pipelineName = model.resourceIdentifier.pipelineName
                    , resourceName = model.resourceIdentifier.resourceName
                    , versionID = versionID
                    }

                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (.id >> (==) versionID)

                newExpandedState : Bool
                newExpandedState =
                    case version of
                        Just v ->
                            not v.expanded

                        Nothing ->
                            False
            in
            ( updateVersion versionID (\v -> { v | expanded = newExpandedState }) model
            , if newExpandedState then
                [ FetchInputTo versionedResourceIdentifier
                , FetchOutputOf versionedResourceIdentifier
                ]

              else
                []
            )

        InputToFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin ] )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        InputToFetched versionID (Ok builds) ->
            ( updateVersion versionID (\v -> { v | inputTo = builds }) model
            , []
            )

        OutputOfFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin ] )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        ClockTick now ->
            ( { model | now = Just now }, [] )

        OutputOfFetched versionID (Ok builds) ->
            ( updateVersion versionID (\v -> { v | outputOf = builds }) model
            , []
            )

        NavTo url ->
            ( model, [ NewUrl url ] )

        TogglePinBarTooltip ->
            ( { model
                | showPinBarTooltip =
                    case model.pinnedVersion of
                        PinnedStaticallyTo _ ->
                            not model.showPinBarTooltip

                        _ ->
                            False
              }
            , []
            )

        ToggleVersionTooltip ->
            let
                pinnedVersionID : Maybe Int
                pinnedVersionID =
                    model.versions.content
                        |> List.Extra.find (.version >> hasPinnedVersion model)
                        |> Maybe.map .id

                newModel =
                    case ( model.pinnedVersion, pinnedVersionID ) of
                        ( PinnedStaticallyTo _, Just id ) ->
                            updateVersion id (\v -> { v | showTooltip = not v.showTooltip }) model

                        _ ->
                            model
            in
            ( newModel, [] )

        PinVersion versionID ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (\v -> v.id == versionID)

                cmd : List Effect
                cmd =
                    case version of
                        Just v ->
                            [ DoPinVersion
                                { teamName = model.resourceIdentifier.teamName
                                , pipelineName = model.resourceIdentifier.pipelineName
                                , resourceName = model.resourceIdentifier.resourceName
                                , versionID = v.id
                                }
                                model.csrfToken
                            ]

                        Nothing ->
                            []

                newModel =
                    { model | pinnedVersion = Pinned.startPinningTo versionID model.pinnedVersion }
            in
            ( newModel
            , cmd
            )

        UnpinVersion ->
            let
                cmd : List Effect
                cmd =
                    [ DoUnpinVersion
                        { teamName = model.resourceIdentifier.teamName
                        , pipelineName = model.resourceIdentifier.pipelineName
                        , resourceName = model.resourceIdentifier.resourceName
                        }
                        model.csrfToken
                    ]
            in
            ( { model | pinnedVersion = Pinned.startUnpinning model.pinnedVersion }, cmd )

        VersionPinned (Ok ()) ->
            let
                newPinnedVersion =
                    Pinned.finishPinning
                        (\pinningTo ->
                            model.versions.content
                                |> List.Extra.find (\v -> v.id == pinningTo)
                                |> Maybe.map .version
                        )
                        model.pinnedVersion
            in
            ( { model | pinnedVersion = newPinnedVersion }, [] )

        VersionPinned (Err _) ->
            ( { model
                | pinnedVersion = NotPinned
              }
            , []
            )

        VersionUnpinned (Ok ()) ->
            ( { model
                | pinnedVersion = NotPinned
              }
            , []
            )

        VersionUnpinned (Err _) ->
            ( { model
                | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion
              }
            , []
            )

        ToggleVersion action versionID ->
            ( updateVersion versionID (\v -> { v | enabled = BoolTransitionable.Changing }) model
            , [ DoToggleVersion action
                    { teamName = model.resourceIdentifier.teamName
                    , pipelineName = model.resourceIdentifier.pipelineName
                    , resourceName = model.resourceIdentifier.resourceName
                    , versionID = versionID
                    }
                    model.csrfToken
              ]
            )

        VersionToggled action versionID result ->
            let
                newEnabledState : BoolTransitionable.BoolTransitionable
                newEnabledState =
                    case ( result, action ) of
                        ( Ok (), Models.Enable ) ->
                            BoolTransitionable.True

                        ( Ok (), Models.Disable ) ->
                            BoolTransitionable.False

                        ( Err _, Models.Enable ) ->
                            BoolTransitionable.False

                        ( Err _, Models.Disable ) ->
                            BoolTransitionable.True
            in
            ( updateVersion versionID (\v -> { v | enabled = newEnabledState }) model
            , []
            )

        PinIconHover state ->
            ( { model | pinIconHover = state }, [] )

        Hover hovered ->
            ( { model | hovered = hovered }, [] )

        TopBarMsg msg ->
            ( TopBar.update msg model |> Tuple.first
            , [ DoTopBarUpdate msg model ]
            )

        Check ->
            case model.userState of
                UserStateLoggedIn _ ->
                    ( { model | checkStatus = Models.CurrentlyChecking }
                    , [ DoCheck model.resourceIdentifier model.csrfToken ]
                    )

                _ ->
                    ( model, [ RedirectToLogin ] )

        Checked result ->
            case result of
                Ok () ->
                    ( { model | checkStatus = Models.CheckingSuccessfully }
                    , [ FetchResource model.resourceIdentifier
                      , FetchVersionedResources
                            model.resourceIdentifier
                            model.currentPage
                      ]
                    )

                Err err ->
                    ( { model | checkStatus = Models.FailingToCheck }
                    , case err of
                        Http.BadStatus { status } ->
                            if status.code == 401 then
                                [ RedirectToLogin ]

                            else
                                [ FetchResource model.resourceIdentifier ]

                        _ ->
                            []
                    )


updateVersion : Int -> (Models.Version -> Models.Version) -> Model -> Model
updateVersion versionID updateFunc model =
    let
        newVersionsContent : List Models.Version
        newVersionsContent =
            model.versions.content
                |> List.Extra.updateIf (.id >> (==) versionID) updateFunc

        versions : Paginated Models.Version
        versions =
            model.versions
    in
    { model | versions = { versions | content = newVersionsContent } }


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
    Html.div
        [ style
            [ ( "-webkit-font-smoothing", "antialiased" )
            , ( "font-weight", "700" )
            ]
        ]
        [ Html.map TopBarMsg <| Html.fromUnstyled <| TopBar.view model
        , subpageView model
        ]


subpageView : Model -> Html Msg
subpageView model =
    if model.pageStatus == Err Models.Empty then
        Html.div [] []

    else
        let
            previousButtonEvent =
                case model.versions.pagination.previousPage of
                    Nothing ->
                        Noop

                    Just pp ->
                        LoadPage pp

            nextButtonEvent =
                case model.versions.pagination.nextPage of
                    Nothing ->
                        Noop

                    Just np ->
                        let
                            updatedPage =
                                { np
                                    | limit = 100
                                }
                        in
                        LoadPage updatedPage

            lastCheckedView =
                case ( model.now, model.lastChecked ) of
                    ( Just now, Just date ) ->
                        viewLastChecked now date

                    ( _, _ ) ->
                        Html.text ""

            headerHeight =
                60
        in
        Html.div []
            [ Html.div
                [ css
                    [ Css.height <| Css.px headerHeight
                    , Css.position Css.fixed
                    , Css.top <| Css.px Styles.pageHeaderHeight
                    , Css.displayFlex
                    , Css.alignItems Css.stretch
                    , Css.width <| Css.pct 100
                    , Css.zIndex <| Css.int 1
                    , Css.backgroundColor <| Css.hex "2a2929"
                    ]
                ]
                [ Html.h1
                    [ css
                        [ Css.fontWeight <| Css.int 700
                        , Css.marginLeft <| Css.px 18
                        , Css.displayFlex
                        , Css.alignItems Css.center
                        , Css.justifyContent Css.center
                        ]
                    ]
                    [ Html.text model.name ]
                , Html.div
                    [ css
                        [ Css.displayFlex
                        , Css.alignItems Css.center
                        , Css.justifyContent Css.center
                        , Css.marginLeft (Css.px 24)
                        ]
                    ]
                    [ lastCheckedView ]
                , pinBar model
                , Html.div
                    [ id "pagination"
                    , style
                        [ ( "display", "flex" )
                        , ( "align-items", "stretch" )
                        ]
                    ]
                    [ case model.versions.pagination.previousPage of
                        Nothing ->
                            Html.div
                                [ style chevronContainer ]
                                [ Html.div
                                    [ style <|
                                        chevron
                                            { direction = "left"
                                            , enabled = False
                                            , hovered = False
                                            }
                                    ]
                                    []
                                ]

                        Just page ->
                            Html.div
                                [ style chevronContainer
                                , onClick previousButtonEvent
                                , onMouseEnter <| Hover Models.PreviousPage
                                , onMouseLeave <| Hover Models.None
                                ]
                                [ Html.a
                                    [ href <|
                                        paginationRoute
                                            model.resourceIdentifier
                                            page
                                    , attribute "aria-label" "Previous Page"
                                    , style <|
                                        chevron
                                            { direction = "left"
                                            , enabled = True
                                            , hovered = model.hovered == Models.PreviousPage
                                            }
                                    ]
                                    []
                                ]
                    , case model.versions.pagination.nextPage of
                        Nothing ->
                            Html.div
                                [ style chevronContainer ]
                                [ Html.div
                                    [ style <|
                                        chevron
                                            { direction = "right"
                                            , enabled = False
                                            , hovered = False
                                            }
                                    ]
                                    []
                                ]

                        Just page ->
                            Html.div
                                [ style chevronContainer
                                , onClick nextButtonEvent
                                , onMouseEnter <| Hover Models.NextPage
                                , onMouseLeave <| Hover Models.None
                                ]
                                [ Html.a
                                    [ href <|
                                        paginationRoute
                                            model.resourceIdentifier
                                            page
                                    , attribute "aria-label" "Next Page"
                                    , style <|
                                        chevron
                                            { direction = "right"
                                            , enabled = True
                                            , hovered = model.hovered == Models.NextPage
                                            }
                                    ]
                                    []
                                ]
                    ]
                ]
            , Html.div
                [ css
                    [ Css.padding3
                        (Css.px <|
                            headerHeight
                                + Styles.pageHeaderHeight
                                + 10
                        )
                        (Css.px 10)
                        (Css.px 10)
                    ]
                ]
                [ checkSection model
                , viewVersionedResources model
                ]
            ]


checkSection :
    { a
        | checkStatus : Models.CheckStatus
        , checkSetupError : String
        , checkError : String
        , hovered : Models.Hoverable
        , userState : UserState
        , teamName : String
    }
    -> Html Msg
checkSection ({ checkStatus, checkSetupError, checkError } as model) =
    let
        failingToCheck =
            checkStatus == Models.FailingToCheck

        checkMessage =
            case checkStatus of
                Models.FailingToCheck ->
                    "checking failed"

                Models.CurrentlyChecking ->
                    "currently checking"

                Models.CheckingSuccessfully ->
                    "checking successfully"

        stepBody =
            if failingToCheck then
                if not (String.isEmpty checkSetupError) then
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkSetupError ]
                        ]
                    ]

                else
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkError ]
                        ]
                    ]

            else
                []

        statusIcon =
            case checkStatus of
                Models.CurrentlyChecking ->
                    Html.fromUnstyled <|
                        Spinner.spinner "14px"
                            [ Html.Attributes.style
                                [ ( "margin", "7px" )
                                ]
                            ]

                _ ->
                    Html.div
                        [ style <|
                            Resource.Styles.checkStatusIcon failingToCheck
                        ]
                        []

        statusBar =
            Html.div
                [ style
                    [ ( "display", "flex" )
                    , ( "justify-content", "space-between" )
                    , ( "align-items", "center" )
                    , ( "flex-grow", "1" )
                    , ( "height", "28px" )
                    , ( "background", Colors.sectionHeader )
                    , ( "padding-left", "5px" )
                    ]
                ]
                [ Html.h3 [] [ Html.text checkMessage ]
                , statusIcon
                ]

        checkBar =
            Html.div
                [ style [ ( "display", "flex" ) ] ]
                [ checkButton model, statusBar ]
    in
    Html.div [ class "resource-check-status" ] <| checkBar :: stepBody


checkButton :
    { a
        | hovered : Models.Hoverable
        , userState : UserState
        , teamName : String
        , checkStatus : Models.CheckStatus
    }
    -> Html Msg
checkButton { hovered, userState, teamName, checkStatus } =
    let
        enabled =
            case userState of
                UserStateLoggedIn user ->
                    case Dict.get teamName user.teams of
                        Just roles ->
                            List.member "member" roles
                                || List.member "owner" roles

                        Nothing ->
                            False

                _ ->
                    True

        isHovered =
            checkStatus
                == Models.CurrentlyChecking
                || (enabled && hovered == Models.CheckButton)
    in
    Html.div
        ([ style
            [ ( "height", "28px" )
            , ( "width", "28px" )
            , ( "background-color", Colors.sectionHeader )
            , ( "margin-right", "5px" )
            , ( "cursor"
              , if isHovered && checkStatus /= Models.CurrentlyChecking then
                    "pointer"

                else
                    "default"
              )
            ]
         , onMouseEnter <| Hover Models.CheckButton
         , onMouseLeave <| Hover Models.None
         ]
            ++ (if enabled then
                    [ onClick Check ]

                else
                    []
               )
        )
        [ Html.div
            [ style
                [ ( "height", "20px" )
                , ( "width", "20px" )
                , ( "margin", "4px" )
                , ( "background-image"
                  , "url(/public/images/baseline-refresh-24px.svg)"
                  )
                , ( "background-position", "50% 50%" )
                , ( "background-repeat", "no-repeat" )
                , ( "background-size", "contain" )
                , ( "opacity"
                  , if isHovered then
                        "1"

                    else
                        "0.5"
                  )
                ]
            ]
            []
        ]


pinBar :
    { a
        | pinnedVersion : ResourcePinState Concourse.Version Int
        , showPinBarTooltip : Bool
        , pinIconHover : Bool
    }
    -> Html Msg
pinBar { pinnedVersion, showPinBarTooltip, pinIconHover } =
    let
        pinBarVersion =
            Pinned.stable pinnedVersion

        attrList : List ( Html.Attribute Msg, Bool ) -> List (Html.Attribute Msg)
        attrList =
            List.filter Tuple.second >> List.map Tuple.first

        isPinnedStatically =
            case pinnedVersion of
                PinnedStaticallyTo _ ->
                    True

                _ ->
                    False

        isPinnedDynamically =
            case pinnedVersion of
                PinnedDynamicallyTo _ ->
                    True

                _ ->
                    False
    in
    Html.div
        (attrList
            [ ( id "pin-bar", True )
            , ( style <| Resource.Styles.pinBar { isPinned = ME.isJust pinBarVersion }, True )
            , ( onMouseEnter TogglePinBarTooltip, isPinnedStatically )
            , ( onMouseLeave TogglePinBarTooltip, isPinnedStatically )
            ]
        )
        ([ Html.div
            (attrList
                [ ( id "pin-icon", True )
                , ( style <|
                        Resource.Styles.pinIcon
                            { isPinned = ME.isJust pinBarVersion
                            , isPinnedDynamically = isPinnedDynamically
                            , hover = pinIconHover
                            }
                  , True
                  )
                , ( onClick UnpinVersion, isPinnedDynamically )
                , ( onMouseEnter <| PinIconHover True, isPinnedDynamically )
                , ( onMouseLeave <| PinIconHover False, True )
                ]
            )
            []
         ]
            ++ (case pinBarVersion of
                    Just v ->
                        [ viewVersion v ]

                    _ ->
                        []
               )
            ++ (if showPinBarTooltip then
                    [ Html.div
                        [ id "pin-bar-tooltip"
                        , style Resource.Styles.pinBarTooltip
                        ]
                        [ Html.text "pinned in pipeline config" ]
                    ]

                else
                    []
               )
        )


checkForVersionID : Int -> Concourse.VersionedResource -> Bool
checkForVersionID versionID versionedResource =
    versionID == versionedResource.id


viewVersionedResources :
    { a
        | versions : Paginated Models.Version
        , pinnedVersion : ResourcePinState Concourse.Version Int
    }
    -> Html Msg
viewVersionedResources { versions, pinnedVersion } =
    versions.content
        |> List.map
            (\v ->
                viewVersionedResource
                    { version = v
                    , pinnedVersion = pinnedVersion
                    }
            )
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { version : Models.Version
    , pinnedVersion : ResourcePinState Concourse.Version Int
    }
    -> Html Msg
viewVersionedResource { version, pinnedVersion } =
    let
        pinState =
            case Pinned.pinState version.version version.id pinnedVersion of
                PinnedStatically _ ->
                    PinnedStatically { showTooltip = version.showTooltip }

                x ->
                    x
    in
    Html.li
        (case ( pinState, version.enabled ) of
            ( Disabled, _ ) ->
                [ style [ ( "opacity", "0.5" ) ] ]

            ( _, BoolTransitionable.False ) ->
                [ style [ ( "opacity", "0.5" ) ] ]

            _ ->
                []
        )
        ([ Html.div
            [ css
                [ Css.displayFlex
                , Css.margin2 (Css.px 5) Css.zero
                ]
            ]
            [ viewEnabledCheckbox
                { enabled = version.enabled
                , id = version.id
                , pinState = pinState
                }
            , viewPinButton
                { versionID = version.id
                , pinState = pinState
                , showTooltip = version.showTooltip
                }
            , viewVersionHeader
                { id = version.id
                , version = version.version
                , pinnedState = pinState
                }
            ]
         ]
            ++ (if version.expanded then
                    [ viewVersionBody
                        { inputTo = version.inputTo
                        , outputOf = version.outputOf
                        , metadata = version.metadata
                        }
                    ]

                else
                    []
               )
        )


viewVersionBody :
    { a
        | inputTo : List Concourse.Build
        , outputOf : List Concourse.Build
        , metadata : Concourse.Metadata
    }
    -> Html Msg
viewVersionBody { inputTo, outputOf, metadata } =
    Html.div
        [ css
            [ Css.displayFlex
            , Css.padding2 (Css.px 5) (Css.px 10)
            ]
        ]
        [ Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "inputs to" ] ]
                , viewBuilds <| listToMap inputTo
                ]
        , Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "outputs of" ] ]
                , viewBuilds <| listToMap outputOf
                ]
        , Html.div [ class "vri metadata-container" ]
            [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
            , viewMetadata metadata
            ]
        ]


viewEnabledCheckbox :
    { a
        | enabled : BoolTransitionable.BoolTransitionable
        , id : Int
        , pinState : VersionPinState
    }
    -> Html Msg
viewEnabledCheckbox { enabled, id, pinState } =
    let
        baseAttrs =
            [ Html.Styled.Attributes.attribute
                "aria-label"
                "Toggle Resource Version Enabled"
            , css
                [ Css.marginRight <| Css.px 5
                , Css.width <| Css.px 25
                , Css.height <| Css.px 25
                , Css.float Css.left
                , Css.backgroundColor <| Css.hex "#1e1d1d"
                , Css.backgroundRepeat Css.noRepeat
                , Css.backgroundPosition2 (Css.pct 50) (Css.pct 50)
                ]
            , style [ ( "cursor", "pointer" ) ]
            ]
                ++ (case pinState of
                        PinnedStatically _ ->
                            [ style
                                [ ( "border", "1px solid " ++ Colors.pinned ) ]
                            ]

                        PinnedDynamically ->
                            [ style
                                [ ( "border", "1px solid " ++ Colors.pinned ) ]
                            ]

                        _ ->
                            []
                   )
    in
    case enabled of
        BoolTransitionable.True ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "background-image"
                              , "url(/public/images/checkmark-ic.svg)"
                              )
                            ]
                       , onClick <| ToggleVersion Models.Disable id
                       ]
                )
                []

        BoolTransitionable.Changing ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "justify-content", "center" )
                            ]
                       ]
                )
                [ Html.fromUnstyled <|
                    Spinner.spinner
                        "12.5px"
                        [ Html.Attributes.style [ ( "margin", "6.25px" ) ] ]
                ]

        BoolTransitionable.False ->
            Html.div
                (baseAttrs ++ [ onClick <| ToggleVersion Models.Enable id ])
                []


viewPinButton :
    { versionID : Int
    , pinState : VersionPinState
    , showTooltip : Bool
    }
    -> Html Msg
viewPinButton { versionID, pinState } =
    let
        baseAttrs =
            [ Html.Styled.Attributes.attribute
                "aria-label"
                "Pin Resource Version"
            , css
                [ Css.position Css.relative
                , Css.backgroundRepeat Css.noRepeat
                , Css.backgroundPosition2 (Css.pct 50) (Css.pct 50)
                , Css.marginRight (Css.px 5)
                , Css.width (Css.px 25)
                , Css.height (Css.px 25)
                , Css.float Css.left
                ]
            ]
    in
    case pinState of
        Enabled ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "background-color", "#1e1d1d" )
                            , ( "cursor", "pointer" )
                            , ( "background-image"
                              , "url(/public/images/pin-ic-white.svg)"
                              )
                            ]
                       , onClick <| PinVersion versionID
                       ]
                )
                []

        PinnedDynamically ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "background-color", "#1e1d1d" )
                            , ( "cursor", "pointer" )
                            , ( "background-image"
                              , "url(/public/images/pin-ic-white.svg)"
                              )
                            , ( "border", "1px solid " ++ Colors.pinned )
                            ]
                       , onClick UnpinVersion
                       ]
                )
                []

        PinnedStatically { showTooltip } ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "background-color", "#1e1d1d" )
                            , ( "cursor", "default" )
                            , ( "background-image"
                              , "url(/public/images/pin-ic-white.svg)"
                              )
                            , ( "border", "1px solid " ++ Colors.pinned )
                            ]
                       , onMouseOut ToggleVersionTooltip
                       , onMouseOver ToggleVersionTooltip
                       ]
                )
                (if showTooltip then
                    [ Html.div
                        [ css
                            [ Css.position Css.absolute
                            , Css.bottom <| Css.px 25
                            , Css.backgroundColor <| Css.hex "9b9b9b"
                            , Css.zIndex <| Css.int 2
                            , Css.padding <| Css.px 5
                            , Css.width <| Css.px 170
                            ]
                        ]
                        [ Html.text "enable via pipeline config" ]
                    ]

                 else
                    []
                )

        Disabled ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "background-color", "#1e1d1d" )
                            , ( "cursor", "default" )
                            , ( "background-image"
                              , "url(/public/images/pin-ic-white.svg)"
                              )
                            ]
                       ]
                )
                []

        InTransition ->
            Html.div
                (baseAttrs
                    ++ [ style
                            [ ( "background-color", "#1e1d1d" )
                            , ( "cursor", "default" )
                            , ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "justify-content", "center" )
                            ]
                       ]
                )
                [ Html.fromUnstyled <|
                    Spinner.spinner
                        "12.5px"
                        [ Html.Attributes.style [ ( "margin", "6.25px" ) ] ]
                ]


viewVersionHeader : { a | id : Int, version : Concourse.Version, pinnedState : VersionPinState } -> Html Msg
viewVersionHeader { id, version, pinnedState } =
    Html.div
        ([ css
            [ Css.flexGrow <| Css.num 1
            , Css.backgroundColor <| Css.hex "1e1d1d"
            , Css.cursor Css.pointer
            , Css.displayFlex
            , Css.alignItems Css.center
            , Css.paddingLeft <| Css.px 10
            , Css.color <| Css.hex <| "e6e7e8"
            ]
         , onClick <| ExpandVersionedResource id
         ]
            ++ (case pinnedState of
                    PinnedStatically _ ->
                        [ style [ ( "border", "1px solid " ++ Colors.pinned ) ] ]

                    PinnedDynamically ->
                        [ style [ ( "border", "1px solid " ++ Colors.pinned ) ] ]

                    _ ->
                        []
               )
        )
        [ viewVersion version ]


viewVersion : Concourse.Version -> Html Msg
viewVersion version =
    (Html.fromUnstyled << DictView.view)
        << Dict.map (\_ s -> Html.toUnstyled (Html.text s))
    <|
        version


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


viewLastChecked : Time -> Date -> Html a
viewLastChecked now date =
    let
        ago =
            Duration.between (Date.toTime date) now
    in
    Html.table []
        [ Html.tr
            []
            [ Html.td [] [ Html.text "checked" ]
            , Html.td [ title (Date.Format.format "%b %d %Y %I:%M:%S %p" date) ]
                [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
            ]
        ]


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
                        [ Html.Styled.Attributes.fromUnstyled <| StrictEvents.onLeftClick <| NavTo link
                        , href link
                        ]
                        [ Html.text <| "#" ++ build.name ]
                    ]
    in
    [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
    , Html.ul [ class "builds-list" ]
        (case Dict.get jobName buildDict of
            Nothing ->
                []

            -- never happens
            Just buildList ->
                List.map oneBuildToLi buildList
        )
    ]


updateExpandedProperties : Model -> List Effect
updateExpandedProperties model =
    let
        filteredList =
            List.filter
                (isExpanded model.versions.content)
                model.versions.content
    in
    List.concatMap
        (Effects.fetchInputAndOutputs model)
        filteredList


isExpanded : List Models.Version -> Models.Version -> Bool
isExpanded versions version =
    versions
        |> List.Extra.find (.id >> (==) version.id)
        |> Maybe.map .expanded
        |> Maybe.withDefault False


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every (5 * Time.second) AutoupdateTimerTicked
        , Time.every Time.second ClockTick
        ]
